FROM golang:1.25

WORKDIR /app
COPY . .
ENV SERVER_PORT = 8080 \
    STORAGE_MODE = mongo \
    MONGO_URL = mongodb://localhost:27017 \
    MONGO_DBNAME = posts

RUN go build -o server .
EXPOSE 8080

CMD ["./server"]