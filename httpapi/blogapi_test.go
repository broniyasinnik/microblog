package httpapi

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"micro-blog/microblog"
	"net/http"
	"os"
	"strconv"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	openapi3_routers "github.com/getkin/kin-openapi/routers"
	openapi3_legacy "github.com/getkin/kin-openapi/routers/legacy"
	"github.com/stretchr/testify/suite"
)

//go:embed microblog.yaml
var apiSpec []byte

var ctx = context.Background()

type APISuite struct {
	suite.Suite
	client http.Client

	apiSpecRouter openapi3_routers.Router
}

func TestAPI(t *testing.T) {
	suite.Run(t, &APISuite{})
}

func (s *APISuite) SetupSuite() {
	manager := microblog.NewInMemoryManager()
	srv := NewServer(manager)
	go func() {
		err := srv.ListenAndServe()
		log.Fatal(err)
	}()
	spec, err := openapi3.NewLoader().LoadFromData(apiSpec)
	s.Require().NoError(err)
	s.Require().NoError(spec.Validate(ctx))
	router, err := openapi3_legacy.NewRouter(spec)
	s.Require().NoError(err)
	s.apiSpecRouter = router
	s.client.Transport = s.specValidating(http.DefaultTransport)

}

func (s *APISuite) createPostRequest(userId string, createReq *CreatePostRequest) *http.Request {
	body, err := json.Marshal(createReq)
	s.Require().NoError(err)
	req, err := http.NewRequest("POST", "http://localhost:8080/api/v1/posts", bytes.NewReader(body))
	s.Require().NoError(err)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("System-Design-User-Id", userId)
	return req
}

func (s *APISuite) TestCreatePost() {
	s.Run("CreatePostWithBadUserName", func() {
		userId := "broniy93!"
		createReq := &CreatePostRequest{Text: "Hello World!"}
		req := s.createPostRequest(userId, createReq)
		resp, _ := s.client.Do(req)
		s.Require().Equal(http.StatusUnauthorized, resp.StatusCode)
	})
	s.Run("CreatePost", func() {
		userId := "abc123"
		text := "My First Post!"
		createReq := &CreatePostRequest{Text: text}
		req := s.createPostRequest(userId, createReq)
		resp, err := s.client.Do(req)
		s.Require().NoError(err)

		var createResp *CreatePostResponse
		err = json.NewDecoder(resp.Body).Decode(&createResp)
		s.Require().NoError(err)

		s.Require().Equal(createResp.AuthorId, userId)
		s.Require().Equal(createResp.Text, text)

	})
}

func (s *APISuite) setupUserPosts(userId string, numPosts int) []CreatePostResponse {
	posts := make([]CreatePostResponse, numPosts)
	for i := 1; i <= numPosts; i++ {
		postText := fmt.Sprintf("I'm user %s and this is post number %d!", userId, i)
		post := &CreatePostRequest{Text: postText}
		req := s.createPostRequest(userId, post)
		resp, err := s.client.Do(req)
		s.Require().NoError(err)
		err = json.NewDecoder(resp.Body).Decode(&posts[numPosts-i])
		s.Require().NoError(err)
	}
	return posts
}

func (s *APISuite) TestGetPost() {
	posts := s.setupUserPosts("abc123", 3)
	s.Run("GetNotExistPost", func() {
		resp, _ := s.client.Get("http://localhost:8080/api/v1/posts/post4")
		s.Require().Equal(http.StatusNotFound, resp.StatusCode)
	})

	s.Run("GetPost", func() {
		var body CreatePostResponse
		for i := 0; i < 3; i++ {
			resp, _ := s.client.Get(fmt.Sprintf("http://localhost:8080/api/v1/posts/%s", posts[i].PostId))
			err := json.NewDecoder(resp.Body).Decode(&body)
			s.Require().NoError(err)
			s.Require().Equal(http.StatusOK, resp.StatusCode)
			s.Require().Equal(posts[i], body)
		}
	})
}

func (s *APISuite) TestGetPosts() {
	userId := "abc123"
	numPosts := 5
	posts := s.setupUserPosts(userId, numPosts)

	s.Run("GetFirstPage", func() {
		var postsResp GetPostsResponse
		resp, _ := s.client.Get("http://localhost:8080/api/v1/users/" + userId + "/posts?size=2")
		err := json.NewDecoder(resp.Body).Decode(&postsResp)
		s.Require().NoError(err)
		s.Require().Len(postsResp.Posts, 2)
		for i := 0; i < 2; i++ {
			s.Require().Equal(posts[i], postsResp.Posts[i])
		}
	})
	s.Run("GetAllPages", func() {
		var postsResp GetPostsResponse
		pageSize := 3
		for i := 0; i < len(posts); i += pageSize {
			url := "http://localhost:8080/api/v1/users/" + userId + "/posts"
			req, _ := http.NewRequest(http.MethodGet, url, nil)
			q := req.URL.Query()
			q.Add("size", strconv.Itoa(pageSize))
			if postsResp.NextPage != "" {
				q.Add("page", postsResp.NextPage)
			}
			req.URL.RawQuery = q.Encode()
			resp, _ := s.client.Do(req)
			err := json.NewDecoder(resp.Body).Decode(&postsResp)
			s.Require().NoError(err)
			for cnt := 0; cnt < pageSize && i+cnt < len(posts); cnt++ {
				s.Require().Equal(postsResp.Posts[cnt], posts[i+cnt])
			}
		}
	})
}

func (s *APISuite) specValidating(transport http.RoundTripper) http.RoundTripper {
	return RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		log.Println("Send HTTP request:")
		reqBody := s.printReq(req)

		// validate request
		route, params, err := s.apiSpecRouter.FindRoute(req)
		s.Require().NoError(err)
		reqDescriptor := &openapi3filter.RequestValidationInput{
			Request:     req,
			PathParams:  params,
			QueryParams: req.URL.Query(),
			Route:       route,
		}
		s.Require().NoError(openapi3filter.ValidateRequest(ctx, reqDescriptor))

		// do request
		req.Body = io.NopCloser(bytes.NewReader(reqBody))
		resp, err := transport.RoundTrip(req)
		if err != nil {
			return nil, err
		}
		log.Println("Got HTTP response:")
		respBody := s.printResp(resp)

		// Validate response against OpenAPI spec
		s.Require().NoError(openapi3filter.ValidateResponse(ctx, &openapi3filter.ResponseValidationInput{
			RequestValidationInput: reqDescriptor,
			Status:                 resp.StatusCode,
			Header:                 resp.Header,
			Body:                   io.NopCloser(bytes.NewReader(respBody)),
		}))

		return resp, nil
	})
}

func (s *APISuite) printReq(req *http.Request) []byte {
	body := s.readAll(req.Body)

	req.Body = io.NopCloser(bytes.NewReader(body))
	s.Require().NoError(req.Write(os.Stdout))
	fmt.Println()

	req.Body = io.NopCloser(bytes.NewReader(body))
	return body
}

func (s *APISuite) printResp(resp *http.Response) []byte {
	body := s.readAll(resp.Body)

	resp.Body = io.NopCloser(bytes.NewReader(body))
	s.Require().NoError(resp.Write(os.Stdout))
	fmt.Println()

	resp.Body = io.NopCloser(bytes.NewReader(body))
	return body
}

func (s *APISuite) readAll(in io.Reader) []byte {
	if in == nil {
		return nil
	}
	data, err := io.ReadAll(in)
	s.Require().NoError(err)
	return data
}

type RoundTripperFunc func(*http.Request) (*http.Response, error)

func (fn RoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}
