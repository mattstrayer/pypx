FROM node:24.15.0-alpine@sha256:d1b3b4da11eefd5941e7f0b9cf17783fc99d9c6fc34884a665f40a06dbdfc94f AS builder
WORKDIR /app
COPY pnpm-workspace.yaml pnpm-lock.yaml ./
COPY web/package.json web/pnpm-lock.yaml ./web/
RUN corepack enable && pnpm install --frozen-lockfile
COPY . .
RUN cd web && CI=true pnpm build

FROM node:24.15.0-alpine@sha256:d1b3b4da11eefd5941e7f0b9cf17783fc99d9c6fc34884a665f40a06dbdfc94f
WORKDIR /app
COPY --from=builder /app/web/.output .output
EXPOSE 3000
CMD ["node", ".output/server/index.mjs"]
