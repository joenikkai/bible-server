# bible-server

this is an webswever api for the guttenberg bible embeded in go

## how to run 

**build**
```sh
docker build -t bible-server .
```
**run**
```sh
docker run -p 7000:7000 -e PORT=7000 bible-server
```

Call the api on port `7000`