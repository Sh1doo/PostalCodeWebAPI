#ベースイメージ設定
FROM golang:1.20

#作業ディレクトリ指定
WORKDIR /app

#依存関係のファイルをコピー
COPY cmd/go.mod cmd/go.sum ./

#必要なモジュールをダウンロード
RUN go mod download

#ソースコードをコピー
COPY cmd/main.go ./

#ビルドの実行
RUN CGO_ENABLED=0 GOOS=linux go build

#リッスンするポート
EXPOSE 8080

#実行
CMD ["go", "run", "main.go"]