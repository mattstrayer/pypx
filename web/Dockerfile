FROM node:26.13.0-alpine@sha256:8996bb5821d62a8fd9bb27c1cb07462f3fdce6e47d640c6a881ead0b064ce216 AS builder
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

FROM node:26.13.0-alpine@sha256:8996bb5821d62a8fd9bb27c1cb07462f3fdce6e47d640c6a881ead0b064ce216
WORKDIR /app
# The runtime only runs node, so drop the package managers the base image ships.
RUN rm -rf /usr/local/lib/node_modules /usr/local/bin/npm /usr/local/bin/npx /usr/local/bin/corepack
COPY --from=builder /app/web/.output .output
EXPOSE 3000
CMD ["node", ".output/server/index.mjs"]
