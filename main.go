package main

import (
        "database/sql"
        "encoding/json"
        "log"
        "net/http"
        "os"
        "time"

        "github.com/julienschmidt/httprouter"
        _ "github.com/lib/pq"
        "golang.org/x/crypto/bcrypt"
)

// Структуры данных
type Author struct {
        ID       int    `json:"id"`
        Username string `json:"username"`
        Email    string `json:"email"`
        Password string `json:"-"`
}

type Article struct {
        ID        int       `json:"id"`
        AuthorID  int       `json:"author_id"`
        Title     string    `json:"title"`
        Content   string    `json:"content"`
        CreatedAt time.Time `json:"created_at"`
}

type Metrics struct {
        TotalArticles int `json:"total_articles"`
        TotalAuthors  int `json:"total_authors"`
}

var db *sql.DB

func main() {
        // Подключение к PostgreSQL
        connStr := "user=blogadmin dbname=blog password=secretpassword sslmode=disable"
        var err error
        db, err = sql.Open("postgres", connStr)
        if err != nil {
                log.Fatal(err)
        }
        defer db.Close()

        // Проверка подключения
        err = db.Ping()
        if err != nil {
                log.Fatal(err)
        }

        // Инициализация БД
        initDB()

        // Роутер
        router := httprouter.New()
        router.GET("/", listArticles)
        router.POST("/register", registerAuthor)
        router.POST("/articles", createArticle)
        router.GET("/metrics", getMetrics)

        port := os.Getenv("PORT")
        if port == "" {
                port = "8080"
        }

        log.Printf("Сервер запущен на порту :%s", port)
        log.Fatal(http.ListenAndServe(":"+port, router))
}

func initDB() {
        // Создание таблицы авторов
        _, err := db.Exec(`
                CREATE TABLE IF NOT EXISTS authors (
                        id SERIAL PRIMARY KEY,
                        username VARCHAR(50) UNIQUE NOT NULL,
                        email VARCHAR(100) UNIQUE NOT NULL,
                        password VARCHAR(100) NOT NULL
                )
        `)
        if err != nil {
                log.Fatal(err)
        }

        // Создание таблицы статей
        _, err = db.Exec(`
                CREATE TABLE IF NOT EXISTS articles (
                        id SERIAL PRIMARY KEY,
                        author_id INTEGER REFERENCES authors(id) NOT NULL,
                        title VARCHAR(255) NOT NULL,
                        content TEXT NOT NULL,
                        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
                )
        `)
        if err != nil {
                log.Fatal(err)
        }
}

// Обработчики
func listArticles(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
        rows, err := db.Query(`
                SELECT a.id, a.title, a.content, a.created_at, 
                        au.id as author_id, au.username
                FROM articles a
                JOIN authors au ON a.author_id = au.id
                ORDER BY a.created_at DESC
                LIMIT 100
        `)
        if err != nil {
                http.Error(w, err.Error(), http.StatusInternalServerError)
                return
        }
        defer rows.Close()

        articles := []struct {
                Article
                AuthorName string `json:"author_name"`
        }{}

        for rows.Next() {
                var a Article
                var authorName string
                err := rows.Scan(
                        &a.ID,
                        &a.Title,
                        &a.Content,
                        &a.CreatedAt,
                        &a.AuthorID,
                        &authorName,
                )
                if err != nil {
                        http.Error(w, err.Error(), http.StatusInternalServerError)
                        return
                }

                articles = append(articles, struct {
                        Article
                        AuthorName string `json:"author_name"`
                }{a, authorName})
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(articles)
}

func registerAuthor(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
        var author struct {
                Username string `json:"username"`
                Email    string `json:"email"`
                Password string `json:"password"`
        }

        err := json.NewDecoder(r.Body).Decode(&author)
        if err != nil {
                http.Error(w, err.Error(), http.StatusBadRequest)
                return
        }

        // Хеширование пароля
        hashedPassword, err := bcrypt.GenerateFromPassword(
                []byte(author.Password), bcrypt.DefaultCost)
        if err != nil {
                http.Error(w, err.Error(), http.StatusInternalServerError)
                return
        }

        // Сохранение в БД
        var id int
        err = db.QueryRow(`
                INSERT INTO authors (username, email, password)
                VALUES ($1, $2, $3)
                RETURNING id
        `, author.Username, author.Email, string(hashedPassword)).Scan(&id)

        if err != nil {
                http.Error(w, err.Error(), http.StatusInternalServerError)
                return
        }

        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusCreated)
        json.NewEncoder(w).Encode(map[string]interface{}{
                "id":       id,
                "username": author.Username,
        })
}

func createArticle(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
        // Базовая аутентификация
        username, password, ok := r.BasicAuth()
        if !ok {
                http.Error(w, "Требуется авторизация", http.StatusUnauthorized)
                return
        }

        // Проверка учетных данных
        var author Author
        err := db.QueryRow(`
                SELECT id, password FROM authors WHERE username = $1
        `, username).Scan(&author.ID, &author.Password)

        if err != nil {
                http.Error(w, "Неверные учетные данные", http.StatusUnauthorized)
                return
        }

        err = bcrypt.CompareHashAndPassword(
                []byte(author.Password), []byte(password))
        if err != nil {
                http.Error(w, "Неверные учетные данные", http.StatusUnauthorized)
                return
        }

        // Чтение данных статьи
        var article struct {
                Title   string `json:"title"`
                Content string `json:"content"`
        }

        err = json.NewDecoder(r.Body).Decode(&article)
        if err != nil {
                http.Error(w, err.Error(), http.StatusBadRequest)
                return
        }

        // Сохранение статьи
        var id int
        err = db.QueryRow(`
                INSERT INTO articles (author_id, title, content)
                VALUES ($1, $2, $3)
                RETURNING id
        `, author.ID, article.Title, article.Content).Scan(&id)

        if err != nil {
                http.Error(w, err.Error(), http.StatusInternalServerError)
                return
        }

        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusCreated)
        json.NewEncoder(w).Encode(map[string]interface{}{
                "id":    id,
                "title": article.Title,
        })
}

func getMetrics(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
        metrics := Metrics{}

        // Получение метрик из БД
        err := db.QueryRow("SELECT COUNT(*) FROM articles").Scan(&metrics.TotalArticles)
        if err != nil {
                http.Error(w, err.Error(), http.StatusInternalServerError)
                return
        }

        err = db.QueryRow("SELECT COUNT(*) FROM authors").Scan(&metrics.TotalAuthors)
        if err != nil {
                http.Error(w, err.Error(), http.StatusInternalServerError)
                return
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(metrics)
}
