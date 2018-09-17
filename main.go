package main

import (
	"fmt"
	"log"
	"mime"
	"net/http"
	"os"

	"github.com/go-redis/redis"
)

const (
	// FormatHeader name of the header used to extract the format
	FormatHeader = "X-Format"

	// CodeHeader name of the header used as source of the HTTP statu code to return
	CodeHeader = "X-Code"

	// ContentType name of the header that defines the format of the reply
	ContentType = "Content-Type"

	// OriginalURI name of the header with the original URL from NGINX
	OriginalURI = "X-Original-URI"

	// Namespace name of the header that contains information about the Ingress namespace
	Namespace = "X-Namespace"

	// IngressName name of the header that contains the matched Ingress
	IngressName = "X-Ingress-Name"

	// ServiceName name of the header that contains the matched Service in the Ingress
	ServiceName = "X-Service-Name"

	// ServicePort name of the header that contains the matched Service port in the Ingress
	ServicePort = "X-Service-Port"

	// ErrFilesPathVar is the name of the environment variable indicating
	// the location on disk of files served by the handler.
	ErrFilesPathVar = "ERROR_FILES_PATH"
)

// Getter represents the ability to get some value from a datastore
type Getter interface {
	Get(key string) *struct{}
}

func main() {
	errFilesPath := "/www"
	if os.Getenv(ErrFilesPathVar) != "" {
		errFilesPath = os.Getenv(ErrFilesPathVar)
	}

	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	http.HandleFunc("/", errorHandler(client))

	// http.Handle("/metrics", promhttp.Handler())

	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	http.ListenAndServe(fmt.Sprintf(":8080"), nil)
}

func errorHandler(client *redis.Client) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		ext := "html"

		ingressName := r.Header.Get(IngressName)
		namespace := r.Header.Get(Namespace)

		if os.Getenv("DEBUG") != "" {
			log.Printf("FormatHeader: %s", r.Header.Get(FormatHeader))
			log.Printf("CodeHeader: %s", r.Header.Get(CodeHeader))
			log.Printf("ContentType: %s", r.Header.Get(ContentType))
			log.Printf("OriginalURI: %s", r.Header.Get(OriginalURI))
			log.Printf("Namespace: %s", r.Header.Get(Namespace))
			log.Printf("IngressName: %s", r.Header.Get(IngressName))
			log.Printf("ServiceName: %s", r.Header.Get(ServiceName))
			log.Printf("ServicePort: %s", r.Header.Get(ServicePort))
			// w.Header().Set(FormatHeader)
			// w.Header().Set(CodeHeader)
			// w.Header().Set(ContentType, r.Header.Get(ContentType))
			// w.Header().Set(OriginalURI, r.Header.Get(OriginalURI))
			// w.Header().Set(Namespace, r.Header.Get(Namespace))
			// w.Header().Set(IngressName, r.Header.Get(IngressName))
			// w.Header().Set(ServiceName, r.Header.Get(ServiceName))
			// w.Header().Set(ServicePort, r.Header.Get(ServicePort))
		}

		format := r.Header.Get(FormatHeader)
		if format == "" {
			format = "text/html"
			log.Printf("format not specified. Using %v", format)
		}

		cext, err := mime.ExtensionsByType(format)
		if err != nil {
			log.Printf("unexpected error reading media type extension: %v. Using %v", err, ext)
		} else if len(cext) == 0 {
			log.Printf("couldn't get media type extension. Using %v", ext)
		} else {
			ext = cext[0]
		}
		w.Header().Set(ContentType, format)

		if ingressName != "" {
			_, err := client.Get(fmt.Sprintf("sleeping:%s:%s", namespace, ingressName)).Result()
			if err == redis.Nil {
				log.Println("App is not sleeping")
			} else if err != nil {
				panic(err)
			}
			fmt.Fprintf(w, "App %s is sleeping!", r.Header.Get(IngressName))
			return
		}
		fmt.Fprint(w, "Page not found - 404")
	}
}
