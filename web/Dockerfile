FROM node:24.21.0-alpine@sha256:ebfe2f90462722a7a4de65e91990e97fe0d401c70e0e762c5b53302f905ec1c1 AS builder
WORKDIR /app
COPY pnpm-workspace.yaml pnpm-lock.yaml ./
COPY web/package.json ./web/
RUN corepack enable && pnpm install --frozen-lockfile
COPY . .
RUN cd web && CI=true pnpm build

FROM node:24.21.0-alpine@sha256:ebfe2f90462722a7a4de65e91990e97fe0d401c70e0e762c5b53302f905ec1c1
WORKDIR /app
COPY --from=builder /app/web/.output .output
EXPOSE 3000
CMD ["node", ".output/server/index.mjs"]
