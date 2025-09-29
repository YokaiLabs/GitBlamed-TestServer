package main

import (
	"archive/tar"
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"testing/fstest"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

//go:embed image/* tests/*
var files embed.FS

type Code struct {
	User string `json:"user"`
	Code string `json:"code"`
	Task string `json:"task"`
}

func createFS(task string, code string) fstest.MapFS {
	memFS := fstest.MapFS{
		"code.ts": &fstest.MapFile{Data: []byte(code), Mode: 0644},
	}

	dockerfile, err := files.ReadFile("image/Dockerfile")
	if err != nil {
		panic(err)
	}

	testFile, err := files.ReadFile(fmt.Sprintf("tests/%s.ts", task))
	if err != nil {
		panic(err)
	}

	memFS["Dockerfile"] = &fstest.MapFile{Data: dockerfile, Mode: 0644}
	memFS["test.ts"] = &fstest.MapFile{Data: testFile, Mode: 0644}

	return memFS
}

func tarImageContext(files fs.FS) (io.Reader, error) {
	buffer := new(bytes.Buffer)
	tarwriter := tar.NewWriter(buffer)

	err := fs.WalkDir(files, ".", func(file string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}

		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = file
		hdr.ModTime = time.Now()

		if err := tarwriter.WriteHeader(hdr); err != nil {
			return err
		}

		if entry.IsDir() {
			return nil
		}

		f, err := files.Open(file)
		if err != nil {
			return err
		}
		defer f.Close()
		if _, err := io.Copy(tarwriter, f); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	if err := tarwriter.Close(); err != nil {
		return nil, err
	}

	return buffer, nil
}

func executeCodeTest(code string, task string, user string) []byte {
	ctx := context.Background()

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(fmt.Errorf("opening client %e", err))
	}

	imageName := fmt.Sprintf("%s-%s-test", user, task)
	fmt.Printf("building %s", imageName)
	imageContext, err := tarImageContext(createFS(task, code))
	if err != nil {
		panic(fmt.Errorf("creating image rar %e", err))
	}

	buildOutput, err := cli.ImageBuild(ctx, imageContext, client.ImageBuildOptions{Tags: []string{imageName}, Dockerfile: "/Dockerfile", Remove: false})
	if err != nil {
		panic(fmt.Errorf("building image", err))
	}

	defer buildOutput.Body.Close()

	_, err = io.ReadAll(buildOutput.Body)
	if err != nil {
		fmt.Printf("error reading body %e", err)
	}

	containerOutput, err := cli.ContainerCreate(ctx, &container.Config{
		Image: imageName,
	}, nil, nil, nil, "")
	if err != nil {
		fmt.Printf("error creating container %e", err)
	}

	err = cli.ContainerStart(ctx, containerOutput.ID, client.ContainerStartOptions{})
	if err != nil {
		fmt.Printf("error starting container %e", err)
	}

	waitChannel, errorChannel := cli.ContainerWait(ctx, containerOutput.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errorChannel:
		{
			fmt.Printf("error running container %e", err)
		}
	case <-waitChannel:
	}

	output, err := cli.ContainerLogs(ctx, containerOutput.ID, client.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	})
	if err != nil {
		fmt.Printf("error getting logs %e", err)
	}

	defer output.Close()

	io.Copy(os.Stdout, output)

	logs, err := io.ReadAll(output)

	if err != nil {
		fmt.Printf("error reading logs %e", err)
	}

	return logs
}

func main() {
	router := http.ServeMux{}

	router.HandleFunc("OPTIONS /", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, Authorization")
		w.WriteHeader(http.StatusOK)
	})

	router.HandleFunc("POST /run", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		body, err := io.ReadAll(r.Body)
		if err != nil {
			fmt.Printf("Error reading body: %e", err)
			return
		}

		defer r.Body.Close()

		code := &Code{}

		err = json.Unmarshal(body, code)
		if err != nil {
			fmt.Printf("Error reading body: %e", err)
			return
		}

		output := executeCodeTest(code.Code, code.Task, code.User)

		w.WriteHeader(200)
		w.Write(output)
	})

	http.ListenAndServe(":8086", &router)
}
