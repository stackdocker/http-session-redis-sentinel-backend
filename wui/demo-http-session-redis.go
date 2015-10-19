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
    "net/http"
    "html/template"
    "log"
    "regexp"
    "strings"
    "encoding/gob"
    "io/ioutil"
    "os"
    "bytes"
    "github.com/spf13/pflag"
    "github.com/gorilla/mux"
    "github.com/gorilla/sessions"
    _ "github.com/gorilla/securecookie"
    redisbackendhttpsessionstore "github.com/stackdocker/http-session-redis-sentinel-backend"
    "github.com/boj/redistore"
)

var (
    buf bytes.Buffer
	logger *log.Logger = log.New(&buf, "logger: ", log.Lshortfile)    
	//logger := log.New(os.Stdout, "logger: ", log.Lshortfile)

    sentinelMode bool = false
    redisAddress string
    conf redisbackendhttpsessionstore.SentinelClientConfig = 
        redisbackendhttpsessionstore.SentinelClientConfig{}

    indexRegex = regexp.MustCompile(`^index\.html?$`)
    validPath = regexp.MustCompile(`^/(signup|signin|signout|profile)/([\S]+)$`)

    templates = make(map[string]*template.Template)

    store sessions.Store
)

func connectSentinel() (*redisbackendhttpsessionstore.SentinelFailoverStore, error) {
    //sentinelstore := redisbackendhttpsessionstore.NewSentinelFailoverStore(
    //    redisbackendhttpsessionstore.SentinelClientConfig{
    //        MasterName: "mymaster",
    //        Addresses: []string{"104.155.238.248:26379","104.155.202.124:26379"},
    //    }, []byte("something-very-secret"))
    sentinelstore := redisbackendhttpsessionstore.NewSentinelFailoverStore(
        conf, []byte("something-very-secret"))

    pong, err := sentinelstore.FailoverClient.Ping().Result()
    if err != nil {
        return nil, err
    }
    fmt.Println(pong)
    return sentinelstore, nil
}

func main() {
    pflag.BoolVar(&sentinelMode, "sentinel-mode", false,
        "Whether using Sentinel")
    pflag.StringVar(&redisAddress, "redis-addr", ":6379", 
        "Redis address, can be ignored while setting sentinel mode")
    pflag.StringVar(&conf.MasterName, "master-name", "mymaster", 
        "Redis Sentinel master name")
    pflag.StringSliceVar(&conf.Addresses, "sentinel-ips", 
        []string{"172.31.33.2:26379", "172.31.33.3:26379", "172.31.75.4:26379"}, 
        "Sentinel failover addresses")
    pflag.Parse()

    if sentinelMode {
        fmt.Println("block to wait connection")
        var err error
        //go func() {store, err = connectSentinel()}()
        store, err = connectSentinel()
        if err != nil {
            fmt.Println("Failed to create connection! Please contact SysOps")
            return
        }
    } else {
        //store = sessions.NewCookieStore([]byte("something-very-secret"))
        //store = sessions.NewFilesystemStore("", []byte("something-very-secret"))
        //store, err := redistore.NewRediStore(10, "tcp", ".6379", "", []byte("secret-key"))
        redistore, err := redistore.NewRediStore(10, "tcp", 
            redisAddress, "", []byte("authentication-secret-key"))
        if err != nil {
            fmt.Println("Failed to ping Redis! Please contact SysOps")
            return 
        }
        store = redistore
    }
    
    gob.Register(&Person{})
    
    port := os.Getenv("PORT")
    if port == "" {
        port = "80"
    }
    
    router := mux.NewRouter()
    router.HandleFunc("/signup/{signup}", makeHandler(signupHandler)).Methods("GET", "POST")
    router.HandleFunc("/signin/{signin}", makeHandler(signinHandler)).Methods("GET", "POST")
    router.HandleFunc("/profile/{profile}", makeHandler(profileHandler)).Methods("GET", "POST")
    router.HandleFunc("/signout/{signout}", makeHandler(signoutHandler)).Methods("GET", "POST")
    //router.HandleFunc("/index.html", makeHandler(indexHandler)).Methods("GET") // substituted by following statement
    router.HandleFunc("/{others}", func(w http.ResponseWriter, r *http.Request){
        vars := mux.Vars(r)
        others := vars["others"]
        if m := indexRegex.FindStringSubmatch(strings.ToLower(others)); m != nil {
            indexHandler(w, r, others[:len("index")])
            return
        }
        http.NotFound(w, r)
    }).Methods("GET")
    router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request){
        if r.URL.Path == "/" {
            http.Redirect(w, r, "/index.html", http.StatusFound)
        }
    }).Methods("GET")

    http.Handle("/", router)
    loadTemplates()
    
    fmt.Printf("Listening on port %s\n", port)
    log.Fatal(http.ListenAndServe(":" + port, nil))
}


// page controller

func loadTemplates() {
    templates["index-login.html"] = template.Must(
        template.ParseFiles("tmpl/index-login.html", "tmpl/index-content.html", 
        "tmpl/base.html"))
    templates["index-logout.html"] = template.Must(
        template.ParseFiles("tmpl/index-logout.html", "tmpl/signout.html", 
        "tmpl/index-content.html", "tmpl/base.html"))
    templates["signup.html"] = template.Must(
        template.ParseFiles("tmpl/signup.html", "tmpl/base.html"))
    templates["signin.html"] = template.Must(
        template.ParseFiles("tmpl/signin.html", "tmpl/base.html"))
    templates["profile.html"] = template.Must(
        template.ParseFiles("tmpl/profile.html", "tmpl/signout.html", 
        "tmpl/base.html"))
    templates["bye.html"] = template.Must(
        template.ParseFiles("tmpl/bye.html", "tmpl/base.html"))
}

func renderTemplate(w http.ResponseWriter, tmpl string, p *Page) {
    err := templates[tmpl + ".html"].ExecuteTemplate(w, "base", p)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
    }
}

func makeHandler(fn func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        m := validPath.FindStringSubmatch(r.URL.Path)
        if m == nil {
            //if c := strings.EqualFold(r.URL.Path, "/index.html"); c {
            //    fn(w, r, "index")
            //    return
            //}
            http.NotFound(w, r)
            return
        }
        fn(w, r, m[2])
    }
}

func indexHandler(w http.ResponseWriter, r *http.Request, title string) {
    buf.Reset()
	logger.Print(r.Method, " ", r.URL.Path, " ", title)

    // Get a session.
    session, err := store.Get(r, "session-name")
    if err != nil {
        logger.Print("exception=", err.Error())
        http.Error(w, err.Error(), http.StatusInternalServerError)
        fmt.Print(&buf)
        return
    }
/*
    // Set some session values.
    session.Values["foo"] = "bar"
    session.Values[42] = 43
    
    // Get the previously flashes, if any.
    if flashes := session.Flashes(); len(flashes) > 0 {
        // Use the flash values.
    } else {
        // Set a new flash.
        session.AddFlash("Hello, flash messages world!")
    }
*/
    // viewer
    p, err := loadPage("static/index")
    if err != nil {
        logger.Print("exception=", err.Error())
        http.Error(w, err.Error(), http.StatusInternalServerError)
        fmt.Print(&buf)
        return
    }

    if session.IsNew {
        logger.Print("Output an unauthorized home page")
        //not authenticated, signup or signin
        p.Title = "Welcome!"
    
        // Save it before we write to the response/return from the handler.
        session.Save(r, w)    
        renderTemplate(w, "index-login", p)    
        
        fmt.Print(&buf)
        return
    }

    // Get the previously flashes, if any.
    flashes := session.Flashes("trace")
    var person *Person
    // Retrieve our struct and type-assert it
    v := session.Values["person"]
    
    // viewer
    person, ok := v.(*Person)
    if !ok {
        logger.Print("anonymous-session=", session.ID)
        // Handle the case that it's not an expected type
        if len(flashes) > 0 {
            // Use the flash values.
            if item, ok := flashes[0].(string); ok {
                p.Title = item
            }
            //if len(flashes) > 1 {
            //    if item, ok := flashes[1].(string); ok {
            //        p.Body = []byte(item)
            //    }
            //}
        }
        session.Save(r, w) 
        renderTemplate(w, "index-login", p)    
        fmt.Print(&buf)
        return
    }
    logger.Print("session=", session.ID)
    p.Title = person.Name
    // Save it before we write to the response/return from the handler.
    session.Save(r, w)    
    renderTemplate(w, "index-logout", p)            
    fmt.Print(&buf)
}

func signupHandler(w http.ResponseWriter, r *http.Request, title string) {
    buf.Reset()
	logger.Print(r.Method, " ", r.URL.Path, " ", title)

    // Get a session.
    session, err := store.Get(r, "session-name")
    if err != nil {
        logger.Print("exception=", err.Error())
        http.Error(w, err.Error(), http.StatusInternalServerError)
        fmt.Print(&buf)
        return
    }
    
    var flashes []interface{}
    var person *Person
    if !session.IsNew {
        logger.Print("watch existed session: ", session.ID)
        // Get the previously flashes, if any.
        flashes = session.Flashes("trace")
        // Retrieve our struct and type-assert it
        v := session.Values["person"]
        person, ok := v.(*Person)
        if ok {
            logger.Print(person)
        }
    }

    switch r.Method {
    case "GET":
        logger.Print("present sign up form")
        p := &Page{Title: title,}
        if len(flashes) > 0 {
            // Use the flash values.
            if title, ok := flashes[0].(string); ok {
                p.Title = title
            }
            if len(flashes) > 1 {
                if body, ok := flashes[1].(string); ok {
                    p.Body = []byte(body)
                }
            }
        }
        renderTemplate(w, "signup", p)
        fmt.Print(&buf)
    case "POST":
        logger.Print("validation with file authentication")
        p, err := loadPage("secret/baseauth")
        if err != nil {
            logger.Print("-> exception=", err.Error())
            http.Error(w, err.Error(), http.StatusInternalServerError)
            fmt.Print(&buf)
            return
        }

        if ok := bytes.Contains(p.Body, []byte(r.FormValue("user") + ":")); ok {
            logger.Print("-> dupicated user name")
            // Set a new flash.
            session.AddFlash("invalid user name, try another!", "trace")
            session.Save(r, w)
            http.Redirect(w, r, "/signup/baseauth", http.StatusFound)
            fmt.Print(&buf)
            return
        }        
        
        p.Body = append(p.Body, []byte("\n" + r.FormValue("user") + ":" + r.FormValue("password"))...)
        if err := p.save(); err != nil {
            logger.Print("-> exception=", err.Error())
            session.AddFlash("Failed to access database, try again!", "trace")
            session.AddFlash(r.FormValue("user"), "trace")
            session.Save(r, w)
            http.Error(w, err.Error(), http.StatusInternalServerError)
            fmt.Print(&buf)
            return
        }
        logger.Print("-> new account: ", r.FormValue("user"))
        if person == nil {
            person = &Person{Id: "staging" , Name: r.FormValue("user"),} 
        } else {
            person.Id = "staging"
            person.Name = r.FormValue("user")
        }
        session.Values["person"] = person
        session.AddFlash("Message to new user!", "trace")
        // Save it before we write to the response/return from the handler.
        session.Save(r, w)
        http.Redirect(w, r, "/profile/baseauth", http.StatusFound)
        fmt.Print(&buf)
    default:
        logger.Print("-> not implementated url")
        fmt.Print(&buf)
    }
}

// Serve:
//     GET /signin/baseauth response a form to auth
//     POST /signin/action check for account
//     GET|POST /signin/redir response a redircte to new url
func signinHandler(w http.ResponseWriter, r *http.Request, title string) {
    buf.Reset()
    logger.Print(r.Method, " ", r.URL.Path, " ", title)
    
    // Get a session.
    session, err := store.Get(r, "session-name")
    if err != nil {
        logger.Print("exception=", err.Error())
        http.Error(w, err.Error(), 500)
        fmt.Print(&buf)
        return
    }
    var flashes []interface{}
    var person *Person
    if !session.IsNew {
        logger.Print("watch existed session: ", session.ID)
        // Get the previously flashes, if any.
        flashes = session.Flashes("trace")
        // Retrieve our struct and type-assert it
        v := session.Values["person"]
        person, ok := v.(*Person)
        if ok {
            logger.Print(person)
        }
    }
    // Page viewer
    p := &Page{Title: title,}
    if r.Method == "GET" && title == "baseauth" {
        logger.Print("build auth form")
        if !session.IsNew {
            if len(flashes) > 0 {
                // Use the flash values.
                if item, ok := flashes[0].(string); ok {
                    p.Title = item
                }
                if len(flashes) > 1 {
                    if item, ok := flashes[1].(string); ok {
                        p.Body = []byte(item)
                    }
                }
            } else if person != nil {
                p.Title = "Are you " + person.Name
                p.Body = []byte(person.Name)
            } 
        }
        session.Save(r, w)
        renderTemplate(w, "signin", p)
        fmt.Print(&buf)
        return
    }
    if r.Method == "POST" && title == "action" {
        logger.Print("play with file authentication")
        cred := r.FormValue("user") + ":" + r.FormValue("password")
        // whether re login
        if person != nil && person.Name != r.FormValue("user") {
            delete(session.Values, "person")
        }
        // Load account data
        a, err := loadPage("secret/baseauth")
        if err != nil {
            logger.Print("-> exception=", err.Error())
            http.Error(w, err.Error(), http.StatusInternalServerError)
            fmt.Print(&buf)
            return
        }
        // Authentication
        if ok := bytes.Contains(a.Body, []byte(cred)); !ok {
            logger.Print("-> Not authenticated")
            // Set a new flash.
            session.AddFlash("Sign failure, try again!", "trace")
            session.AddFlash(r.FormValue("Name"), "trace")
            session.Save(r, w)
            http.Redirect(w, r, "/signin/baseauth", http.StatusFound)
            fmt.Print(&buf)
            return
        }
        logger.Print("Authenticated")
        person = &Person {Id: "staging" , Name: r.FormValue("user")} 
        session.Values["person"] = person
        // Save it before we write to the response/return from the handler.
        session.Save(r, w)
        http.Redirect(w, r, "/signin/redir", http.StatusFound)
        fmt.Print(&buf)
        return
    }
    vars := mux.Vars(r)
    base := vars["signin"]
    if strings.EqualFold(base, "redir") {
        http.Redirect(w, r, "/profile/baseauth", http.StatusFound)
        fmt.Print(&buf)
        return
    }
    // Invalid method and path
    logger.Print("-> Invalid url.")
    http.NotFound(w, r)  
    fmt.Print(&buf)
}

func profileHandler(w http.ResponseWriter, r *http.Request, title string) {
    buf.Reset()
    logger.Print(r.Method, " ", r.URL.Path, " ", title)
    
    // Get a session.
    session, err := store.Get(r, "session-name")
    if err != nil {
        logger.Print("exception=", err.Error())
        http.Error(w, err.Error(), 500)
        fmt.Print(&buf)
        return
    }
    if session.IsNew {
        logger.Print("maybe reach maxage, session=", session.ID)
        http.Redirect(w, r, "/signup/bye", http.StatusFound)
        fmt.Print(&buf)
        return
    }

    var flashes []interface{}
    var person *Person
    flashes = session.Flashes("trace")
    // Retrieve our struct and type-assert it
    v := session.Values["person"]
    person, ok := v.(*Person)
    if !ok {
        logger.Print("maybe login failure, session=", session.ID)
        if len(flashes) > 0 {
            // Use the flash values.
            ephemeral := "ephemeral: "
            if item, ok := flashes[0].(string); ok {
                ephemeral += item
            }
            if len(flashes) > 1 {
                // Use the flash values.
                if item, ok := flashes[1].(string); ok {
                    ephemeral += ", " + item
                }
            }
            logger.Print("-> ", ephemeral)
        }
        http.Redirect(w, r, "/signup/bye", http.StatusFound)
        fmt.Print(&buf)
        return
    }
    logger.Print("Manipulate user profile page -> ", person)
    p := &Page{}
    p.Title = person.Name
    p.Body = []byte(person.Id)
    session.Save(r, w)
    renderTemplate(w, "profile", p)
    fmt.Print(&buf)
}

func signoutHandler(w http.ResponseWriter, r *http.Request, title string) {
    buf.Reset()
	logger.Print(r.Method, " ", r.URL.Path, " ", title)
	
    // Get a session.
    session, err := store.Get(r, "session-name")
    if err != nil {
        logger.Print(err.Error())
        http.Error(w, err.Error(), 500)
        fmt.Print(&buf)
        return
    }
    
    if r.Method == "POST" && title == "baseauth" {
        if session.IsNew {
            logger.Print("Fresh conversation")
            session.Options.MaxAge = 0
            // Save it before we write to the response/return from the handler.
            session.Save(r, w)
            http.Redirect(w, r, "/signout/redir", http.StatusFound)
            fmt.Print(&buf)
            return
        }
        
        flashes := session.Flashes("trace")
        
        v := session.Values["person"]
        person, ok := v.(*Person)
        if !ok {
            // Handle the case that it's not an expected type
            logger.Print("Unexpected! no one logged in.")
            if len(flashes) > 0 {
                ephemeral := "ephemeral: "
                // Use the flash values.
                if item, ok := flashes[0].(string); ok {
                    ephemeral += item
                }
                if len(flashes) > 1 {
                    if item, ok := flashes[1].(string); ok {
                        //p.Body = []byte(item)
                        ephemeral += " " + item 
                    }
                }
                logger.Print("ephemeral=", ephemeral)
            }
        } else {   
            logger.Print("(", person.Name, ") has logged out.")
            delete(session.Values, "person")
        }
        
        session.Options.MaxAge = 0
        // Save it before we write to the response/return from the handler.
        session.Save(r, w)
        
        http.Redirect(w, r, "/signout/bye", http.StatusFound)
        fmt.Print(&buf)
        return
    }
    if title == "bye" {
        logger.Print("byebye!")
        p := &Page{Title: "See you!", }
        var flashes []interface{}
        var person *Person
        flashes = session.Flashes("trace")
        v := session.Values["person"]
        person, ok := v.(*Person)
        if ok {
            logger.Print("user profile -> ", person)
        }
        // Retrieve our struct and type-assert it
        if len(flashes) > 0 {
            // Use the flash values.
            if item, ok := flashes[0].(string); ok {
                p.Title = item
            }
        }
        //session.Save(r, w)
        renderTemplate(w, "bye", p)
        fmt.Print(&buf)
        return
    }
    // Invalid method and path
    logger.Print("invalid url.")
    http.NotFound(w, r)    
    fmt.Print(&buf)
}

/*
* Page Model
*/

type Person struct {
    Id       string
    Name     string
}

type Page struct {
	Title string
	Body  []byte
}

func (p *Page) save() error {
	filename := p.Title + ".txt"
	return ioutil.WriteFile(filename, p.Body, 0600)
}

func loadPage(title string) (*Page, error) {
	filename := title + ".txt"
	body, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return &Page{Title: title, Body: body}, nil
}



