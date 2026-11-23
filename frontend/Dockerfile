FROM node:24-alpine

WORKDIR /workspace

RUN corepack enable

COPY . .

RUN pnpm install --frozen-lockfile

WORKDIR /workspace/frontend

EXPOSE 3000

CMD ["pnpm", "dev", "--hostname", "0.0.0.0"]
