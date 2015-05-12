package nameserver

import (
	"fmt"
	"github.com/gorilla/mux"
	"github.com/miekg/dns"
	. "github.com/weaveworks/weave/common"
	"log"
	"net"
	"net/http"
)

func httpErrorAndLog(level *log.Logger, w http.ResponseWriter, msg string,
	status int, logmsg string, logargs ...interface{}) {
	http.Error(w, msg, status)
	level.Printf("[http] "+logmsg, logargs...)
}

func ListenHTTP(version string, server *DNSServer, domain string, db Zone, port int) {

	muxRouter := mux.NewRouter()

	muxRouter.Methods("GET").Path("/status").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "weave DNS", version)
		fmt.Fprintln(w, server.Status())
	})

	muxRouter.Methods("PUT").Path("/name/{id:.+}/{ip:.+}").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqError := func(msg string, logmsg string, logargs ...interface{}) {
			httpErrorAndLog(Warning, w, msg, http.StatusBadRequest, logmsg, logargs...)
		}

		vars := mux.Vars(r)
		idStr := vars["id"]
		ipStr := vars["ip"]
		name := r.FormValue("fqdn")

		if name == "" {
			reqError("Invalid FQDN", "Invalid FQDN in request: %s, %s", r.URL, r.Form)
			return
		}

		ip := net.ParseIP(ipStr)
		if ip == nil {
			reqError("Invalid IP", "Invalid IP in request: %s", ipStr)
			return
		}

		if dns.IsSubDomain(domain, name) {
			Info.Printf("[http] Adding %s -> %s", name, ipStr)
			if err := db.AddRecord(idStr, name, ip); err != nil {
				if _, ok := err.(DuplicateError); !ok {
					httpErrorAndLog(
						Error, w, "Internal error", http.StatusInternalServerError,
						"Unexpected error from DB: %s", err)
					return
				} // oh, I already know this. whatever.
			}
		} else {
			Info.Printf("[http] Ignoring name %s, not in %s", name, domain)
		}
	})

	muxRouter.Methods("DELETE").Path("/name/{id:.+}/{ip:.+}").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		idStr := vars["id"]
		ipStr := vars["ip"]
		fqdnStr := r.FormValue("fqdn")

		ip := net.ParseIP(ipStr)
		if ip == nil {
			httpErrorAndLog(
				Warning, w, "Invalid IP in request", http.StatusBadRequest,
				"Invalid IP in request: %s", ipStr)
			return
		}

		var fqdn *string
		if fqdnStr != "" {
			fqdn = &fqdnStr
		} else {
			fqdnStr = "*"
		}

		Info.Printf("[http] Deleting ID %s, IP %s, FQDN %s)", idStr, ipStr, fqdnStr)
		db.DeleteRecords(&idStr, &ip, fqdn)
	})

	muxRouter.Methods("DELETE").Path("/name/{id:.+}").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		idStr := vars["id"]
		fqdnStr := r.FormValue("fqdn")

		var fqdn *string
		if fqdnStr != "" {
			fqdn = &fqdnStr
		} else {
			fqdnStr = "*"
		}

		Info.Printf("[http] Deleting ID %s, IP *, FQDN %s", idStr, fqdnStr)
		db.DeleteRecords(&idStr, nil, fqdn)
	})

	muxRouter.Methods("DELETE").Path("/name").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fqdnStr := r.FormValue("fqdn")

		var fqdn *string
		if fqdnStr != "" {
			fqdn = &fqdnStr
		} else {
			fqdnStr = "*"
		}

		Info.Printf("[http] Deleting ID *, IP *, FQDN %s", fqdnStr)
		db.DeleteRecords(nil, nil, fqdn)
	})

	http.Handle("/", muxRouter)

	address := fmt.Sprintf(":%d", port)
	if err := http.ListenAndServe(address, nil); err != nil {
		Error.Fatal("[http] Unable to create http listener: ", err)
	}
}
