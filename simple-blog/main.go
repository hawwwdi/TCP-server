package main

import (
	"fmt"
	"github.com/julienschmidt/httprouter"
	"golang.org/x/crypto/bcrypt"
	"html/template"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var tpl *template.Template
var sessionsMap map[string]string
var usersMap map[string]User

type User struct {
	Id       string
	Password []byte
	IsAdmin  bool
}

func init() {
	sessionsMap = make(map[string]string)
	usersMap = make(map[string]User)
	addNewUser("admin", "admin", true)
	tpl = template.Must(template.ParseGlob("templates/*.html"))
}

func main() {
	fmt.Println("running...")
	mux := httprouter.New()
	//mux.Handler("GET", "/", http.FileServer(http.Dir("./templates")))
	mux.GET("/", index)
	mux.Handler("GET", "/favicon.ico", http.FileServer(http.Dir("")))
	mux.POST("/panel", postLogin)
	mux.GET("/panel", cookieLogin)
	mux.GET("/changePass", showChangePass)
	//	mux.POST("/", changePassword)
	mux.GET("/show/:pic", show)
	mux.POST("/show", showPic)
	//mux.Handler("GET", "/files/",http.StripPrefix("/files", http.FileServer(http.Dir("./"))))
	mux.ServeFiles("/files/*filepath", http.Dir("./"))
	mux.GET("/newUser", showAddUser)
	mux.POST("/addUser", addUser)
	mux.POST("/logout", logOut)
	err := http.ListenAndServe("localhost:8080", mux)
	handleErr(os.Stdout, err)
}

func index(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	if alreadySignedIn(r) {
		http.Redirect(w, r, "/panel", http.StatusSeeOther)
		return
	}
	err1 := tpl.ExecuteTemplate(w, "index.html", nil)
	handleErr(w, err1)
}

func cookieLogin(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	user := getUser(r)
	if user == nil {
		http.Error(w, "invalid cookie", http.StatusBadRequest)
		return
	}
	login(w, user)
}

func postLogin(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	err := r.ParseMultipartForm(64)
	handleErr(w, err)
	username, pass, rememberMe := r.PostFormValue("user"), r.PostFormValue("pass"), r.PostFormValue("rememberMe")
	user, err2 := checkUser(username, pass)
	if err2 != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if rememberMe == "true" && !alreadySignedIn(r) {
		writeSession(w, username)
	}
	login(w, user)
}

func login(w http.ResponseWriter, user *User) {
	if user.IsAdmin {
		http.SetCookie(w, &http.Cookie{
			Name:  "last-seen",
			Value: time.Now().Format("15:04:05"),
		})
		http.SetCookie(w, &http.Cookie{
			Name:  "admin",
			Value: "true",
		})
		data := struct {
			User, Pass, LastSeen string
		}{user.Id, string(user.Password), "Now"}
		err := tpl.ExecuteTemplate(w, "panel.html", data)
		handleErr(w, err)
	} else {
		logoutButton := `<br><form method="post" action="/logout">
                <input type="submit" value="LogOut">
            </form>`
		w.Header().Set("Content-Type", "text/html")
		_, err := fmt.Fprintf(w, "welcome %v %v", user.Id, logoutButton)
		handleErr(w, err)
	}
}

/*func changePassword(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	handleErr(w, r.ParseForm())
	PASS = r.PostFormValue("pass")
	handleErr(w, tpl.ExecuteTemplate(w, "index.html", nil))
}*/

func showChangePass(w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
	handleErr(w, tpl.ExecuteTemplate(w, "change.html", nil))
}

func show(w http.ResponseWriter, r *http.Request, sp httprouter.Params) {
	picName := sp.ByName("pic")
	http.ServeFile(w, r, picName)
}

func showPic(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	file, details, err := r.FormFile("file")
	handleErr(w, err)
	defer file.Close()
	bytes, err1 := ioutil.ReadAll(file)
	handleErr(w, err1)
	toSave, err2 := os.Create(filepath.Join("./files/", details.Filename))
	defer toSave.Close()
	handleErr(w, err2)
	_, err3 := toSave.Write(bytes)
	handleErr(w, err3)
	err = tpl.ExecuteTemplate(w, "show.html", string(bytes))
	handleErr(w, err)
}

func addUser(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	if _, err := r.Cookie("admin"); err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	r.ParseForm()
	username, pass := r.PostFormValue("id"), r.PostFormValue("pass")
	addNewUser(username, pass, false)
	http.SetCookie(w, &http.Cookie{
		Name:   "admin",
		MaxAge: -1,
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
	return
}

func addNewUser(username, pass string, isAdmin bool) {
	encryptedPass, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.MinCost)
	handleErr(os.Stdout, err)
	usersMap[username] = User{
		Id:       username,
		Password: encryptedPass,
		IsAdmin:  isAdmin,
	}
}

func showAddUser(w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
	err := tpl.ExecuteTemplate(w, "addUser.html", nil)
	handleErr(w, err)
}

func handleErr(w io.Writer, err error) {
	if err != nil {
		switch v := w.(type) {
		case http.ResponseWriter:
			http.Error(v, err.Error(), http.StatusInternalServerError)
		case *os.File:
			io.Copy(v, strings.NewReader(err.Error()))
		}
	}
}
