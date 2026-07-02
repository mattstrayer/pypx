FROM node:24.18.0-alpine@sha256:a0b9bf06e4e6193cf7a0f58816cc935ff8c2a908f81e6f1a95432d679c54fbfd AS builder
WORKDIR /app
COPY pnpm-workspace.yaml pnpm-lock.yaml ./
COPY web/package.json ./web/
RUN corepack enable && pnpm install --frozen-lockfile
COPY . .
RUN cd web && CI=true pnpm build

FROM node:24.18.0-alpine@sha256:a0b9bf06e4e6193cf7a0f58816cc935ff8c2a908f81e6f1a95432d679c54fbfd
WORKDIR /app
COPY --from=builder /app/web/.output .output
EXPOSE 3000
CMD ["node", ".output/server/index.mjs"]
