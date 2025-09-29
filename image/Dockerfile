FROM "denoland/deno"

WORKDIR /test

USER deno

COPY code.ts .
COPY test.ts .

RUN deno install --entrypoint test.ts

CMD ["test", "--reporter=junit"]
