FROM node:26.13.0-alpine@sha256:c6ad9e9c1d80f6eb607c26b4b0a132ff7a2344fad1c78ea3baa2a7d4c8de6d94 AS builder
WORKDIR /app
COPY pnpm-workspace.yaml pnpm-lock.yaml ./
COPY web/package.json ./web/
# Node 25+ does not ship corepack. The lockfile in deploy/pnpm pins pnpm by hash.
COPY deploy/pnpm/package.json deploy/pnpm/package-lock.json /opt/pnpm/
RUN npm ci --prefix /opt/pnpm
ENV PATH=/opt/pnpm/node_modules/.bin:$PATH
RUN pnpm install --frozen-lockfile
COPY . .
RUN cd web && CI=true pnpm build

FROM node:26.13.0-alpine@sha256:c6ad9e9c1d80f6eb607c26b4b0a132ff7a2344fad1c78ea3baa2a7d4c8de6d94
WORKDIR /app
COPY --from=builder /app/web/.output .output
EXPOSE 3000
CMD ["node", ".output/server/index.mjs"]
