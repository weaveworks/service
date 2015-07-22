FROM scratch
ADD app /
ADD templates /templates
EXPOSE 3000
CMD ["/app"]
