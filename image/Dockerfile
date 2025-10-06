FROM "denoland/deno"

WORKDIR /test

COPY code.ts .
COPY test.ts .

RUN deno install --entrypoint test.ts

CMD ["test", "--junit-path=report.xml"]
