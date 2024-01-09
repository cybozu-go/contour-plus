FROM scratch
LABEL org.opencontainers.image.source="https://github.com/cybozu-go/contour-plus"

COPY bin/contour-plus /contour-plus
COPY LICENSE          /LICENSE

EXPOSE 8180
USER 10000:10000

ENTRYPOINT ["/contour-plus"]
