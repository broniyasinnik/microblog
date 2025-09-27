package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"micro-blog/microblog"

	"github.com/gorilla/mux"
)

type HTTPHandler struct {
	manager microblog.Manager
}

func NewServer(manager microblog.Manager) *http.Server {
	r := mux.NewRouter()
	handler := HTTPHandler{manager}

	r.HandleFunc("/api/v1/posts", handler.CreatePost).Methods(http.MethodPost)
	r.HandleFunc("/api/v1/posts/{postId}", handler.GetPost).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/users/{userId}/posts", handler.GetPosts).Methods(http.MethodGet)
	r.HandleFunc("/maintenance/ping", handler.CheckIsReady).Methods(http.MethodGet)
	srv := &http.Server{
		Addr:         "0.0.0.0:8080",
		Handler:      r,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	return srv
}

type CreatePostRequest struct {
	Text string `json:"text"`
}
type CreatePostResponse struct {
	PostId    string `json:"id"`
	Text      string `json:"text"`
	AuthorId  string `json:"authorId"`
	CreatedAt string `json:"createdAt"`
}

type GetPostsResponse struct {
	Posts    []CreatePostResponse `json:"posts"`
	NextPage string               `json:"nextPage"`
}

func (h *HTTPHandler) CreatePost(w http.ResponseWriter, r *http.Request) {
	var body CreatePostResponse

	usrId := r.Header.Get("System-Design-User-Id")
	re := regexp.MustCompile(`^[0-9a-f]+$`)
	if !re.MatchString(usrId) {
		http.Error(w, "The user_id is not valid", http.StatusUnauthorized)
		return
	}

	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	post, err := h.manager.AddPost(r.Context(), usrId, body.Text)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	rawResponse, _ := json.Marshal(CreatePostResponse{post.PostId, post.Text, post.AuthorId, post.CreatedAt.Format(time.RFC3339)})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(rawResponse)

}

func (h *HTTPHandler) GetPost(w http.ResponseWriter, r *http.Request) {
	postId := strings.TrimPrefix(r.URL.Path, `/api/v1/posts/`)
	post, err := h.manager.GetPost(r.Context(), postId)
	if err != nil {
		http.Error(w, "Post not found", http.StatusNotFound)
	} else {
		rawResponse, _ := json.Marshal(CreatePostResponse{post.PostId, post.Text, post.AuthorId, post.CreatedAt.Format(time.RFC3339)})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(rawResponse)
	}
}

func (h *HTTPHandler) GetPosts(w http.ResponseWriter, r *http.Request) {
	userId := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, `/api/v1/users/`), "/posts")
	page := r.URL.Query().Get("page")
	var pageSize uint8
	fmt.Sscanf(r.URL.Query().Get("size"), "%d", &pageSize)

	// Let the manager fetch the posts in the relevant page
	posts, nextPage, err := h.manager.GetPostsInPage(r.Context(), userId, page, pageSize)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	} else {
		// Form HTTP response with the results
		var resp GetPostsResponse
		resp.NextPage = nextPage
		for _, post := range posts {
			resp.Posts = append(resp.Posts, CreatePostResponse{post.PostId, post.Text, post.AuthorId, post.CreatedAt.Format(time.RFC3339)})
		}
		rawResponse, _ := json.Marshal(resp)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(rawResponse)
	}
}

func (h *HTTPHandler) CheckIsReady(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}
