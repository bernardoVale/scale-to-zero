package main

import (
	"encoding/json"
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

type ErrorStruct struct {
	message string
	error   string
}

func main() {

	client := redis.NewClient(&redis.Options{
		Addr:     "redis-master.default.svc.cluster.local:6379",
		Password: "rKOsaUDIRK", // no password set
		DB:       0,            // use default DB
	})

	http.HandleFunc("/", errorHandler(client))

	// http.Handle("/metrics", promhttp.Handler())

	http.HandleFunc("/error", backendError())

	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	http.ListenAndServe(fmt.Sprintf(":8080"), nil)
}

func backendError() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Print("Handling error")
		data := ErrorStruct{message: "foo", error: "backend error"}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(data)
	}
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
			// err := json.NewDecoder(r.Body).Decode(input)
			// switch {
			// case err == io.EOF:
			// 	log.Println("No body =/")
			// case err != nil:
			// 	b, err := ioutil.ReadAll(r.Body)
			// 	if err != nil {
			// 		panic(err)
			// 	}
			// 	log.Printf("Data %s", b)
			// }
			// return
			val, err := client.Get(fmt.Sprintf("sleeping:%s:%s", namespace, ingressName)).Result()
			if err != nil {
				if err != redis.Nil {
					panic(err)
				}
				fmt.Fprint(w, "App is sleeping but we didn't know =/")
				return
			}
			switch val {
			case "sleeping":
				fmt.Fprintf(w, "App %s is sleeping. Don't you worry, we will start it for you. It might take a few minutes...", r.Header.Get(IngressName))
				client.Publish("wakeup", fmt.Sprintf("%s/%s", namespace, ingressName)).Err()
				if err != nil {
					panic(err)
				}
				if err := client.Set(fmt.Sprintf("sleeping:%s:%s", namespace, ingressName), "waking_up", 0).Err(); err != nil {
					panic(err)
				}
			case "waking_up":
				fmt.Fprintf(w, "App %s is waking up. Wait a little bit more. It might take a few minutes...", r.Header.Get(IngressName))
			case "awake":
				fmt.Fprintf(w, "App %s is awake, but for some reason you end up here. The app is probably facing some issues.\n", ingressName)
			default:
				fmt.Fprintf(w, "Page not found - 404")
			}
			return
		}
		fmt.Fprintf(w, "Page not found - 404")
	}
}
