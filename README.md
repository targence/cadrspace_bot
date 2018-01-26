```
# docker build
docker build . -t cadrspace_bot

# docker run
docker run -d --restart=always -v="$(pwd)/db.gob:/db.gob" -e=TG_TOKEN=XXXXXXXXXXXXXXXXXXXXXX -e=CAL_KEY=XXXXXXXXXXXXXXXXXXXXXX --name=cadrspace_bot cadrspace_bot

# go run
TG_TOKEN=XXXXXXXXXXXXXXXXXXXXXX CAL_KEY=XXXXXXXXXXXXXXXXXXXXXX go run *.go
```