FROM alpine:latest

WORKDIR /app
COPY bin /app/
RUN chmod +x ./plotbot
CMD ["/bin/sh", "-c", "./plotbot >>/data/log/log.txt 2>&1"]
