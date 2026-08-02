# syntax=docker/dockerfile:1
FROM node:26-alpine AS builder
WORKDIR /app
ARG NEXT_PUBLIC_API_BASE_URL
ARG NEXT_PUBLIC_MIDTRANS_SNAP_URL
ARG NEXT_PUBLIC_MIDTRANS_CLIENT_KEY
ARG NEXT_PUBLIC_GOOGLE_CLIENT_ID
ENV NEXT_PUBLIC_API_BASE_URL=${NEXT_PUBLIC_API_BASE_URL}
ENV NEXT_PUBLIC_MIDTRANS_SNAP_URL=${NEXT_PUBLIC_MIDTRANS_SNAP_URL}
ENV NEXT_PUBLIC_MIDTRANS_CLIENT_KEY=${NEXT_PUBLIC_MIDTRANS_CLIENT_KEY}
ENV NEXT_PUBLIC_GOOGLE_CLIENT_ID=${NEXT_PUBLIC_GOOGLE_CLIENT_ID}
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM node:26-alpine
WORKDIR /app
COPY --from=builder /app/package.json /app/package-lock.json ./
COPY --from=builder /app/.next ./.next
COPY --from=builder /app/node_modules ./node_modules
# public/fonts/**/*.ttf is read directly off disk at request time by
# app/api/admin/certificate-template/route.ts (certificate design save,
# async redesign 2026-08-02) to embed fonts as data URIs in the
# self-contained template HTML — next/font/local's build-time processing
# alone doesn't cover this runtime read.
COPY --from=builder /app/public ./public
ENV NODE_ENV=production
ENV PORT=3000
EXPOSE 3000
CMD ["npx", "next", "start"]
