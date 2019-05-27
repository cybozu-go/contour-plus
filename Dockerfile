FROM scratch

COPY bin/contour-plus /contour-plus
COPY LICENSE          /LICENSE

EXPOSE 8080
USER 10000:10000

ENTRYPOINT ["/contour-plus"]
