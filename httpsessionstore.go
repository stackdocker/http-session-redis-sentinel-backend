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

package redisbackendhttpsessionstore

import (
    "fmt"
    "net/http"
    "strings"
    "encoding/base32"
    "errors"
    "time"
    "github.com/gorilla/securecookie"
    "github.com/gorilla/sessions"
    "github.com/boj/redistore"
    "gopkg.in/redis.v3"
)

// Amount of time for cookies/redis keys to expire.
var sessionExpire = 86400 * 30

// redis sentinel client using go-redis/redis 

type SentinelClientConfig struct {
    MasterName string
    Addresses []string
}

func (c *SentinelClientConfig)newSentinelFailoverClient() *redis.Client {
    // See http://redis.io/topics/sentinel for instructions how to
    // setup Redis Sentinel.
    client := redis.NewFailoverClient(&redis.FailoverOptions{
        MasterName: c.MasterName,
        SentinelAddrs: c.Addresses,
    })
    if pong, err := client.Ping().Result(); err != nil {
        fmt.Println("Sentinel currently unable to response, ", 
                "please try Ping laterly when to access")
    } else {
        fmt.Println(pong)
    }
    return client
}


// SentinelFailoverStore stores sessions in Redis Sentinel Failover.
//
// It also serves as a referece for custom stores.
//
// This store is still experimental and not well tested. Feedback is welcome.
type SentinelFailoverStore struct {
	//Codecs             []securecookie.Codec
	//Options            *sessions.Options // default configuration
	//DefaultMaxAge      int     // default Redis TTL for a MaxAge == 0 session
	*redistore.RediStore
	failoverOption     SentinelClientConfig
	FailoverClient     *redis.Client
	maxLength          int
	keyPrefix          string
	serializer         redistore.SessionSerializer
}

// This function returns a new Redis Sentinel store.
//
// Keys are defined in pairs to allow key rotation, but the common case is
// to set a single authentication key and optionally an encryption key.
//
// The first key in a pair is used for authentication and the second for
// encryption. The encryption key can be set to nil or omitted in the last
// pair, but the authentication key is required in all pairs.
//
// It is recommended to use an authentication key with 32 or 64 bytes.
// The encryption key, if set, must be either 16, 24, or 32 bytes to select
// AES-128, AES-192, or AES-256 modes.
//
// Use the convenience function securecookie.GenerateRandomKey() to create
// strong keys.
func NewSentinelFailoverStore(clientConfig SentinelClientConfig, 
        keyPairs ...[]byte) *SentinelFailoverStore {
	client := clientConfig.newSentinelFailoverClient()
	s := &SentinelFailoverStore{ 
		RediStore: &redistore.RediStore {
		    Codecs: securecookie.CodecsFromPairs(keyPairs...),
		    Options: &sessions.Options{
			    Path:   "/",
			    MaxAge: 86400 * 30,
		    },
            DefaultMaxAge: 60 * 60, // 60 minutes
		    //maxLength:     4096,
		    //keyPrefix:     "session_",
		    //serializer: redistore.GobSerializer{},
	    },
		failoverOption: clientConfig,
		FailoverClient: client,
		maxLength:     4096,
		keyPrefix:     "session_",
		serializer: redistore.GobSerializer{},
	}

    s.SetMaxLength(s.maxLength)
    s.SetKeyPrefix(s.keyPrefix)
    s.SetSerializer(s.serializer)
	s.MaxAge(s.Options.MaxAge)
	return s
}

// MaxLength restricts the maximum length of new sessions to l.
// If l is 0 there is no limit to the size of a session, use with caution.
// The default for a new SentinelFailoverStore is 4096.
func (s *SentinelFailoverStore) MaxLength(l int) {
	for _, c := range s.Codecs {
		if codec, ok := c.(*securecookie.SecureCookie); ok {
			codec.MaxLength(l)
		}
	}
}

// Get returns a session for the given name after adding it to the registry.
//
// It returns a new session if the sessions doesn't exist. Access IsNew on
// the session to check if it is an existing session or a new one.
//
// It returns a new session and an error if the session exists but could
// not be decoded.
func (s *SentinelFailoverStore) Get(r *http.Request, name string) (*sessions.Session, error) {
	return sessions.GetRegistry(r).Get(s, name)
}

// New returns a session for the given name without adding it to the registry.
//
// The difference between New() and Get() is that calling New() twice will
// decode the session data twice, while Get() registers and reuses the same
// decoded session after the first call.
func (s *SentinelFailoverStore) New(r *http.Request, name string) (*sessions.Session, error) {
	session := sessions.NewSession(s, name)
	opts := *s.Options
	session.Options = &opts
	session.IsNew = true
	var err error
	if c, errCookie := r.Cookie(name); errCookie == nil {
		err = securecookie.DecodeMulti(name, c.Value, &session.ID, s.Codecs...)
		if err == nil {
			err = s.load(session)
			if err == nil {
				session.IsNew = false
			}
		}
	}
	return session, err
}

// Save adds a single session to the response.
func (s *SentinelFailoverStore) Save(r *http.Request, w http.ResponseWriter, session *sessions.Session) error {
    if session.Options.MaxAge < 0 {
		if err := s.delete(session); err != nil {
			return err
		}
		http.SetCookie(w, sessions.NewCookie(session.Name(), "", session.Options))
		return nil
	}
	if session.ID == "" {
		// Because the ID is not initialized when newly created, encode it to
		// use alphanumeric characters only.
		session.ID = strings.TrimRight(
			base32.StdEncoding.EncodeToString(
				securecookie.GenerateRandomKey(32)), "=")
	}
	if err := s.save(session); err != nil {
		return err
	}
	
	encoded, err := securecookie.EncodeMulti(session.Name(), session.ID,
		s.Codecs...)
	if err != nil {
		return err
	}
	http.SetCookie(w, sessions.NewCookie(session.Name(), encoded, session.Options))
	return nil
}

// MaxAge sets the maximum age for the store and the underlying cookie
// implementation. Individual sessions can be deleted by setting Options.MaxAge
// = -1 for that session.
func (s *SentinelFailoverStore) MaxAge(age int) {
	s.Options.MaxAge = age

	// Set the maxAge for each securecookie instance.
	for _, codec := range s.Codecs {
		if sc, ok := codec.(*securecookie.SecureCookie); ok {
			sc.MaxAge(age)
		}
	}
}

// save writes encoded session.Values to a file.
func (s *SentinelFailoverStore) save(session *sessions.Session) error {
	//encoded, err := securecookie.EncodeMulti(session.Name(), session.Values, 
	//        s.Codecs...)
	data, err := s.serializer.Serialize(session)
	if err != nil {
		return err
	}
	//filename := filepath.Join(s.path, "session_"+session.ID)
	//fileMutex.Lock()
	//defer fileMutex.Unlock()
	//return ioutil.WriteFile(filename, []byte(encoded), 0600)
	
	if s.maxLength != 0 && len(data) > s.maxLength {
		return errors.New("SessionStore: the value to store is too big")
	}
	age := session.Options.MaxAge
	if age == 0 {
		age = s.DefaultMaxAge
	}
	return s.FailoverClient.Set("session_"+session.ID, data, time.Duration(age) * time.Second).Err()
}

// load reads a key and decodes its content into session.Values.
func (s *SentinelFailoverStore) load(session *sessions.Session) error {
	//filename := filepath.Join(s.path, "session_"+session.ID)
	//fileMutex.RLock()
	//defer fileMutex.RUnlock()
	//fdata, err := ioutil.ReadFile(filename)
	data, err := s.FailoverClient.Get("session_"+session.ID).Bytes()
	if err != nil {
	    return err
	}
	return s.serializer.Deserialize(data, session)
	//if err = securecookie.DecodeMulti(session.Name(), string(fdata),
	//	&session.Values, s.Codecs...); err != nil {
	//	return err
	//}
	//return nil
}

// delete removes keys from redis if MaxAge<0
func (s *SentinelFailoverStore) delete(session *sessions.Session) error {
	//conn := s.Pool.Get()
	//defer conn.Close()
	//if _, err := conn.Do("DEL", s.keyPrefix+session.ID); err != nil {
	//	return err
	//}
	//return nil
	return s.FailoverClient.Del("session_" + session.ID).Err()
}
