package main

import (
	"flag"
	"fmt"
	"net/http"
	"time"

	"github.com/go-redis/redis"
	"github.com/sirupsen/logrus"
)

var wakeupChannel, sleepChannel chan string

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

func main() {

	backendHost := flag.String("host", "redis-master:6379", "backend host url")
	backendPassword := flag.String("password", "npCYPR7uAt", "backend password")

	flag.Parse()

	logrus.Info("Starting default backend")
	client := redis.NewClient(&redis.Options{
		Addr:     *backendHost,
		Password: *backendPassword,
		DB:       0, // use default DB
	})

	http.HandleFunc("/", errorHandler(client))

	// http.Handle("/metrics", promhttp.Handler())

	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	logrus.Info("Starting app with time sleep")
	http.ListenAndServe(fmt.Sprintf(":8080"), nil)
	close(wakeupChannel)
}

func waitForIt(client *redis.Client, namespace, ingress string) bool {
	logrus.Infof("Waiting for awake state of app %s/%s", namespace, ingress)
	timeout := time.After(15 * time.Minute)
	tick := time.Tick(time.Second * 2)
	for {
		select {
		case <-tick:
			val, err := client.Get(fmt.Sprintf("sleeping:%s:%s", namespace, ingress)).Result()
			if err != nil {
				logrus.WithError(err).Errorf("Failed to get app status: %v", err)
			}
			if val == "awake" {
				return true
			}
		case <-timeout:
			return false
		}
	}
}

func errorHandler(client *redis.Client) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// ext := "html"

		ingressName := r.Header.Get(IngressName)
		namespace := r.Header.Get(Namespace)
		redirectTo := r.Header.Get(OriginalURI)
		redirectRequest := false

		// logrus.Infof("Redirect to: %s", redirectTo)
		// logrus.Infof("FormatHeader: %s", r.Header.Get(FormatHeader))
		// logrus.Infof("CodeHeader: %s", r.Header.Get(CodeHeader))
		// logrus.Infof("ContentType: %s", r.Header.Get(ContentType))
		// logrus.Infof("OriginalURI: %s", r.Header.Get(OriginalURI))
		// logrus.Infof("Namespace: %s", r.Header.Get(Namespace))
		// logrus.Infof("IngressName: %s", r.Header.Get(IngressName))
		// logrus.Infof("ServiceName: %s", r.Header.Get(ServiceName))
		// logrus.Infof("ServicePort: %s", r.Header.Get(ServicePort))
		// logrus.Infof("Req: %s%s\n", r.Host, r.URL.Path)
		// if os.Getenv("DEBUG") != "" {

		// }

		// format := r.Header.Get(FormatHeader)
		// if format == "" {
		// 	format = "text/html"
		// 	logrus.Infof("format not specified. Using %v", format)
		// }

		// cext, err := mime.ExtensionsByType(format)
		// if err != nil {
		// 	logrus.Infof("unexpected error reading media type extension: %v. Using %v", err, ext)
		// } else if len(cext) == 0 {
		// 	logrus.Infof("couldn't get media type extension. Using %v", ext)
		// } else {
		// 	ext = cext[0]
		// }
		// w.Header().Set(ContentType, format)

		if ingressName != "" {
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
				err := client.Publish("wakeup", fmt.Sprintf("%s/%s", namespace, ingressName)).Err()
				if err != nil {
					logrus.Errorf("Failed to publish wakeup message: %v", err)
				}
				redirectRequest = waitForIt(client, namespace, ingressName)
				// fmt.Fprintf(w, "App %s is sleeping. Don't you worry, we will start it for you. It might take a few minutes...", r.Header.Get(IngressName))
			case "waking_up":
				redirectRequest = waitForIt(client, namespace, ingressName)
			case "awake":
				redirectRequest = true
			default:
				fmt.Fprintf(w, "Page not found - 404")
			}
			if redirectRequest {
				logrus.Info("Redirecting request")
				http.Redirect(w, r, redirectTo, http.StatusSeeOther)
			}
		}
		// Not collected by custom error
		fmt.Fprintf(w, "Page not found - 404")
	}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func int32Ptr(i int32) *int32 { return &i }
