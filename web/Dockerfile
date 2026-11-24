FROM node:26.13.1-alpine@sha256:6bd9ab43c0ec0190ca7b2d193906cb8b7edf6e7f4afa43ddeccd6452710bffb5 AS builder
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

FROM node:26.13.1-alpine@sha256:6bd9ab43c0ec0190ca7b2d193906cb8b7edf6e7f4afa43ddeccd6452710bffb5
WORKDIR /app
# The runtime only runs node, so drop the package managers the base image ships.
RUN rm -rf /usr/local/lib/node_modules /usr/local/bin/npm /usr/local/bin/npx /usr/local/bin/corepack
COPY --from=builder /app/web/.output .output
EXPOSE 3000
CMD ["node", ".output/server/index.mjs"]
