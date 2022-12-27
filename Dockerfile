FROM alpine:3.8

RUN apk --no-cache add ca-certificates=20191127-r2 libc6-compat=1.1.19-r11

EXPOSE 9091

COPY car-pooling-challenge /
 
ENTRYPOINT [ "/car-pooling-challenge" ]
