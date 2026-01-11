package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"license-server/keygen"
)

const ApiToken = "CHANGE_ME_SECRET"

func main() {
	// ✅ 启动阶段不阻塞、不读 stdin
	if _, err := os.Stat("private.pem"); os.IsNotExist(err) {
		log.Println("🔐 private.pem not found, generating...")
		if err := keygen.GenerateKeyPair(); err != nil {
			log.Fatal(err)
		}
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/", index)
	http.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("OK"))
	})
	http.HandleFunc("/api/generate", generate)

	log.Println("Listening on :" + port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func index(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`
<!doctype html>
<html>
<head><title>激活码生成</title></head>
<body>
<h2>生成激活码</h2>
机器码：<input id="m"><br><br>
到期日：<input id="e" type="date"><br><br>
<button onclick="gen()">生成</button>
<pre id="r"></pre>
<script>
function gen(){
fetch('/api/generate',{
	method:'POST',
	headers:{'Content-Type':'application/json'},
	body:JSON.stringify({
		token:'CHANGE_ME_SECRET',
		machine_id:m.value,
		expiry:e.value
	})
}).then(r=>r.text()).then(t=>r.innerText=t)
}
</script>
</body>
</html>
`))
}

func generate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token     string `json:"token"`
		MachineID string `json:"machine_id"`
		Expiry    string `json:"expiry"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", 400)
		return
	}

	if req.Token != ApiToken {
		http.Error(w, "unauthorized", 403)
		return
	}

	code, err := keygen.GenerateLicense(req.MachineID, req.Expiry)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	w.Write([]byte(code))
}