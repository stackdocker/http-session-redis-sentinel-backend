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
    "strings"
    "gopkg.in/redis.v3"
)

func main() {
    var mastername, eps string = "mymaster", "192.168.0.1:26379,192.168.0.2:26379,192.168.0.3:26379"
    var addresses []string
    
    flag.StringVar(&mastername, "master-name", "mymaster", "Redis Sentinel master name")
    flag.StringVar(&eps, "address", ":26379", "Sentinel cluster addresses")
    flag.Parse()
    addresses = strings.Split(eps, ",") 
    
    fmt.Println("Step 1, ping each Sentinel separately...")
    for _, a := range addresses {
        c := redis.NewClient(&redis.Options{
            Addr:     a,  
            Password: "", // no password set
            DB:       0,  // use default DB
        })
    
        cmd := c.Ping()
        fmt.Println(cmd.Val(), cmd.Err(), "----", cmd.String())    
        c.Close()
    }
    
    fmt.Println("Step 2, ping cluster...")
    client := redis.NewFailoverClient(&redis.FailoverOptions{
        MasterName: mastername,   //"mymaster",
        SentinelAddrs: addresses, // x.y.z.a:26379,x.y.z.b:26379
    })

    pong, err := client.Ping().Result()
    fmt.Println(pong, err)    

    fmt.Println("Step last, simple actions...")

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
