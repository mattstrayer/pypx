FROM node:24-alpine AS builder
WORKDIR /app
COPY package.json pnpm-lock.yaml ./
RUN corepack enable && pnpm install --frozen-lockfile
COPY . .
RUN pnpm build

FROM node:24-alpine
WORKDIR /app
COPY --from=builder /app/.output .output
EXPOSE 3000
CMD ["node", ".output/server/index.mjs"]
