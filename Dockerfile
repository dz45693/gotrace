FROM golang:1.15.6

RUN mkdir -p /app

WORKDIR /app
 
ADD main /app/main

EXPOSE 8080
EXPOSE 9090
 
CMD ["./main"]