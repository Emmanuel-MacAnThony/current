# Multi-stage Next.js build → standalone runtime.
FROM node:20-alpine AS deps
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci

FROM node:20-alpine AS build
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
# NEXT_PUBLIC_WS_URL is left unset so the client falls back to
# ws://localhost:8080/ws — correct because the browser runs on the host, where
# the server's :8080 is published by compose.
RUN npm run build

FROM node:20-alpine AS run
WORKDIR /app
ENV NODE_ENV=production
ENV PORT=3002
ENV HOSTNAME=0.0.0.0
# Standalone output: a minimal server.js plus only the deps it traced.
COPY --from=build /app/public ./public
COPY --from=build /app/.next/standalone ./
COPY --from=build /app/.next/static ./.next/static
EXPOSE 3002
CMD ["node", "server.js"]
