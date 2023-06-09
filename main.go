package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time" // to implement time functions

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/thedevsaddam/renderer"
	mgo "gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

var rnd *renderer.Render
var db *mgo.Database

const (
	hostName       string = "localhost:27071"
	port           string = "8080"
	collectionName string = "todo"
	dbName         string = "todo"
)

type todoModel struct {
	ID        string    `bson:"_id,omitempty"`
	Title     string    `bson:"title"`
	Completed bool      `bson:"completed"`
	CreatedAt time.Time `bson:"created_at"`
}

type todo struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Completed bool      `json:completed"`
	CreatedAt time.Time `json:created_at"`
}

func init() {
	rnd = renderer.New()
	sess, err := mgo.Dial(hostName)
	checkErr(err)
	sess.SetMode(mgo.Monotonic, true)
	db = sess.DB(dbName)
}

func checkErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	err := rnd.Template(w, http.StatusOK, []string{"/static/home.tpl"},nil)
	checkErr(err)
}

func fetchTodo(w http.ResponseWriter, r *http.Request) {
	todos := []todoModel{}
	if err := db.C(collectionName).Find(bson.M{}).All(&todos); err != nil {
		rnd.JSON(w, http.StatusProcessing, renderer.M{
			"message" : "failed to fetch todo",
			"error" : err,
		})
		return
	}
	todoList := []todo{}
	for _, t := range todos {
		todoList = append(todoList, todo{
			ID : t.ID,
			Title : t.Title,
			Completed : t.Completed,
			CreatedAt : t.CreatedAt,
		})
	}
	rnd.JSON(w, http.StatusOK, renderer.M{
		"data" : todoList,
	})
}

func createTodo(w http.ResponseWriter, r *http.Request){
	var t todo
	if err := json.Decoder(r.Body).Decode(&t); err != nil {
		rnd.JSON(w, http.StatusProcessing, err)
		return
	}
	if t.Title == "" {
		rnd.JSON(w, http.StatusBadRequest, renderer.M{
			"message":"The Title field is required",
		})
		return
	}
	tm := todoModel{
		ID: bson.NewObjectId(),
		Title: t.Title,
		Completed: false,
		CreatedAt: time.Now(),
	}
	if err := db.C(collectionName).Insert(&tm); err != nil {
		rnd.JSON(w, http.StatusProcessing, renderer.M{
			"message":"failed to Insert todo into the database",
			"error":err
		})
		return
	}
	rnd.JSON(w, http.StatusOK, renderer.M{
		"message":"Todo Created Successfully",
		"todo_id":tm.ID.Hex(),
	})
}

func deleteTodo(w http.ResponseWriter, r *http.Request){
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if !bson.IsObjectIdHex(id){
		rnd.JSON(w, http.StatusBadRequest, renderer.M{
			"message":"The id is invalid"
		})
		return
	}
	if err := db.C(collectionName).RemoveId(bson.ObjectIdHex(id)); err != nil {
		rnd.JSON(w, http.StatusBadRequest, renderer.M{
			"message":"failed to delete Todo from database",
			"error": err,
		})
		return 
	}
	rnd.JSON(w, http.StatusOK, renderer.M{
		"message":"Todo deleted successfully",
	})
}

func updateTodo(w http.ResponseWriter, r *http.Request){
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if !bson.IsObjectIdHex(id){
		rnd.JSON(w, http.StatusBadRequest, renderer.M{
			"message", "The id is invalid",
		})
		return
	}
	var t todo
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		rnd.JSON(w, http.StatusProcessing, err)
		return
	}
	if t.Title == "" {
		rnd.JSON(w, http.StatusBadRequest, renderer.M{
			"message":"The title field is required",
		})
		return
	}
	if err := db.C(collectionName).Update(
		bson.M{"_id", string(bson.ObjectIdHex(id))},
		bson.M{"title":t.Title, "completed":t.Completed},
	); err != nil {
		rnd.JSON(w, http.StatusProcessing, renderer.M{
			"failed to update todo",
			"error":error
		})
		return
	}
	rnd.JSON(w, http.StatusOK, renderer.M{
		"message":"Todo updated successfully",
	})
}

func main() {
	stopChan := make(chan os.Signal)      // creates a channel of type os.Signal and assigns it to stopChan
	signal.Notify(stopChan, os.Interrupt) //configures the channel to recieve os.Interrupt signals, which are sent when user exits the program
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Get("/", homeHandler)
	r.Mount("/todo", todoHandlers())
	serv := &http.Server{
		Addr:         port,
		Handler:      r,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	go func() {
		log.Println("Listening on port ", port)
		if err := serv.ListenAndServe(); err != nil {
			log.Printf("listen:%s\n", err)
		}
	}()

	<-stopChan
	log.Println("shutting down server")
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	serv.Shutdown(ctx)
	defer func() {
		cancel() // defer a call to the cancel function
		log.Println("Server gracefully shut down")
	}()
}

func todoHandlers() http.Handler {
	rg := chi.NewRouter()
	rg.Group(func(r chi.Router) {
		r.Get("/", fetchTodo)
		r.Post("/", createTodo)
		r.Put("/{id}", updateTodo)
		r.Delete("/{id}", deleteTodo)
	})
	return rg
}
