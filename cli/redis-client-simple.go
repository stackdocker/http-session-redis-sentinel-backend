/*
Copyright 2015 All rights reserved.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main
import (
    "fmt"
    "flag"
    "os"
    "gopkg.in/redis.v3"
)

// Demo functions:
//     ping
//     simple set
//     simple get
//     condition list
func main() {
    var addrCmd, addrEnv, addr string 
    flag.StringVar(&addrCmd, "redis-master", ":6379", "Redis address")
    flag.Parse()
    addrEnv = os.Getenv("REDIS_MASTER_ADDRESS")
    if addrEnv != "" {
        addr = addrEnv
    } else {
        addr = addrCmd
    }
    client := redis.NewClient(&redis.Options{
        Addr:   addr,  // ":6379",
        Password: "", // no password set
        DB:       0,  // use default DB
    })
    
    pong, err := client.Ping().Result()
    fmt.Println(pong, err)    
    cmd := client.Ping()
    fmt.Println(cmd.Val(), cmd.Err(), "----", cmd.String())

    err1 := client.Set("key", "value", 0).Err()
    if err1 != nil {
        panic(err1)
    }
    
    val1, err1 := client.Get("key").Result()
    if err1 != nil {
        panic(err1)
    }
    fmt.Println("key", val1)
    
    val2, err2 := client.Get("key2").Result()
    if err2 == redis.Nil {
        fmt.Println("key2 does not exists")
    } else if err2 != nil {
        panic(err2)
    } else {
        fmt.Println("key2", val2)
    }

    val3, err3 := client.Keys("session_*").Result()
    if err3 == redis.Nil {
        fmt.Println("No such keys")
    } else if err3 != nil {
        panic(err3)
    } else {
        fmt.Println(val3)
    }

    client.Close()
}