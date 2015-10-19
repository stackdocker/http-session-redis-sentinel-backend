# http-session-redis-sentinel-backend
implements along with gorilla/sessions and go-redis/redis, and with boj/redistore nested

## How to import

>`import redisbackendhttpsessionstore "github.com/stackdocker/http-session-redis-sentinel-backend"`

## Redis Sentinel client

* redis_cli

  `sudo apt-get install redis-tools` or ...

* nc 

  like `echo -e 'ping\r\nquit\r\n' | nc 127.0.0.1 26379`
  
## Golang redis and sentinel simple test client

  _cli_ subdir
   
  redis client command example
   
>`tangfx@clschool:~/workspace/goprojects/src/github.com/stackdocker/http-session-redis-sentinel-backend/cli (master) $ go run redis-client-simple.go --redis-master 191.168.0.1:6379`

>`PONG <nil>`

>`PONG <nil> ---- PING: PONG`

>`key value`

>`key2 does not exists`

  sentinel client command example

>`tangfx@clschool:~/workspace/goprojects/src/github.com/stackdocker/http-session-redis-sentinel-backend/cli (master) $ go run sentinel-client-simple.go --address 172.31.33.2:26379,172.31.33.3:26379`

>`Step 1, ping each Sentinel separately...`

>`PONG <nil> ---- PING: PONG`

>`PONG <nil> ---- PING: PONG`

>`Step 2, ping Sentinel cluster...`

## Golang http server demo session with Redis and/or Sentinel backend

   _wui_ subdir

### System environment variable
PORT, default value is 80

### Run from source code 
>`$ PORT=8080 go run demo-http-session-redis.go  --sentinel-mode false --redis-addr 192.168.0.1:6379`

>`Listening on port 8080`

>`logger: demo-http-session-redis.go:182: GET /index.html index`

>`logger: demo-http-session-redis.go:254: session=6B3AHP3RBJ6HBS2KWWFQPN5S6IM4TMT33`

or

>`$ go run demo-http-session-redis.go --sentinel-mode true --master-name mymaster --sentinel-ips 172.31.33.2:26379,172.31.33.3:26379`

>`Waiting Redis Sentiel connection`

>`2015/10/19 17:04:34 redis-sentinel: discovered new "mymaster" sentinel: 10.120.1.3:26379`

>`2015/10/19 17:04:34 redis-sentinel: discovered new "mymaster" sentinel: 10.120.1.4:26379`

>`2015/10/19 17:04:34 redis-sentinel: "mymaster" addr is 10.120.1.5:6379`

>`Sentinel currently unable to response,  please try Ping laterly when to access`

>`2015/10/19 17:06:42 redis-sentinel: "mymaster" addr is 10.120.1.5:6379`

>`Failed to create connection! Please contact SysOps`

* Attention

The unexpected thing showing previous command is underlying go-redis v3 client library will try connect internal Redis address.
and spend a few time to exit.

The case shows program must communicate with Redis and Sentinel cluster, thus should configured with same subnet,
or with DNAT to forward data packages to backend

### How to build

After _go build_ or _go install_, copy _secret, static, tmpl_ sub-directories in where executable is located

* secret dir, hold a authentication file
* static dir, simple content for index.html
* tmpl dir, Golang http/template format 



