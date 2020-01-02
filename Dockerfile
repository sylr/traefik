FROM debian:stable-slim
COPY script/ca-certificates.crt /etc/ssl/certs/
COPY dist/traefik /traefik
RUN ls -lah /
EXPOSE 80
VOLUME ["/tmp"]
ENTRYPOINT ["/traefik"]
