package main

import (
	"bytes"
	"compress/gzip"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

// ================= 全局配置 =================

var SecurityToken = getEnv("SECURITY_TOKEN", "123456")

// ================= 数据结构 =================

type LicenseData struct {
	MachineID string `json:"machine_id"`
	ExpiryUTC int64  `json:"expiry_utc"`
}

type License struct {
	Data      string `json:"data"`
	Signature string `json:"signature"`
}

type GenerateRequest struct {
	Token     string `json:"token"`
	MachineID string `json:"machine_id"`
	Expiry    string `json:"expiry"`
}

// 新增：历史记录结构
type HistoryRecord struct {
	GenerateTime string `json:"generate_time"` // 生成时间
	MachineID    string `json:"machine_id"`    // 机器码
	ExpiryDate   string `json:"expiry_date"`   // 到期时间
}

// ================= 全局存储 (简单的内存+文件存储) =================

var (
	historyList []HistoryRecord
	historyFile = "history.json" // 数据存储文件
	mutex       sync.Mutex       // 互斥锁，防止并发写入冲突
)

// ================= 主程序入口 =================

func main() {
	// 0. 启动时加载历史记录
	loadHistory()

	// 1. 检查私钥
	if os.Getenv("PRIVATE_KEY") == "" {
		log.Println("⚠️  警告: 环境变量 PRIVATE_KEY 未设置！")
	}

	// 2. 注册路由
	http.HandleFunc("/", handleIndex)           // 生成页
	http.HandleFunc("/history", handleHistory)  // 新增：历史记录页
	http.HandleFunc("/api/generate", handleAPI) // API 接口
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("OK"))
	})

	// 3. 启动
	port := getEnv("PORT", "80") // 默认为 80，适配 Deployra
	log.Printf("🚀 服务已启动，监听端口 :%s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}

// ================= HTTP 处理函数 =================

// 1. 生成页面
func handleIndex(w http.ResponseWriter, r *http.Request) {
	htmlContent := `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>激活码生成器</title>
    <style>
        body { font-family: -apple-system, sans-serif; max-width: 600px; margin: 40px auto; padding: 20px; background: #f5f5f7; color: #333; }
        .card { background: white; padding: 30px; border-radius: 12px; box-shadow: 0 4px 12px rgba(0,0,0,0.1); }
        h2 { margin-top: 0; color: #0071e3; }
        .form-group { margin-bottom: 20px; }
        label { display: block; margin-bottom: 8px; font-weight: 600; font-size: 14px; }
        input { width: 100%; padding: 12px; border: 1px solid #d2d2d7; border-radius: 8px; font-size: 16px; box-sizing: border-box; }
        button { width: 100%; padding: 14px; background: #0071e3; color: white; border: none; border-radius: 8px; font-size: 16px; font-weight: 600; cursor: pointer; transition: background 0.2s; }
        button:hover { background: #0077ed; }
        .links { margin-top: 20px; text-align: right; font-size: 14px; }
        a { color: #0071e3; text-decoration: none; }
        #result { margin-top: 25px; padding: 15px; background: #1d1d1f; color: #fff; border-radius: 8px; font-family: monospace; word-break: break-all; display: none; line-height: 1.5; }
    </style>
</head>
<body>
    <div class="card">
        <h2>🔐 激活码生成</h2>

        <div class="form-group">
            <label>鉴权密码</label>
            <input type="password" id="token" placeholder="输入 Token">
        </div>

        <div class="form-group">
            <label>客户机器码</label>
            <input type="text" id="mid" placeholder="输入机器码">
        </div>

        <div class="form-group">
            <label>到期日期</label>
            <input type="date" id="date">
        </div>

        <button onclick="generate()" id="btn">生成激活码</button>
        <div id="result"></div>

        <div class="links">
            <a href="#" onclick="goToHistory(); return false;">📄 查看生成记录</a>
        </div>
    </div>

    <script>
        const tomorrow = new Date();
        tomorrow.setDate(tomorrow.getDate() + 1);
        document.getElementById('date').valueAsDate = tomorrow;

        // 自动填充上次的Token
        const savedToken = localStorage.getItem('license_token');
        if(savedToken) document.getElementById('token').value = savedToken;

        function goToHistory() {
            const t = document.getElementById('token').value;
            if(!t) { alert('请输入鉴权密码查看历史'); return; }
            window.location.href = '/history?token=' + t;
        }

        async function generate() {
            const resDiv = document.getElementById('result');
            const btn = document.getElementById('btn');
            const token = document.getElementById('token').value;

            // 保存 Token 方便下次使用
            localStorage.setItem('license_token', token);

            resDiv.style.display = 'block';
            resDiv.innerText = "生成中...";
            btn.disabled = true;

            try {
                const response = await fetch('/api/generate', {
                    method: 'POST',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify({
                        token: token,
                        machine_id: document.getElementById('mid').value,
                        expiry: document.getElementById('date').value
                    })
                });
                const text = await response.text();
                if (response.ok) {
                    resDiv.innerText = text;
                } else {
                    resDiv.innerText = "❌ 错误: " + text;
                }
            } catch (err) {
                resDiv.innerText = "❌ 请求失败: " + err;
            } finally {
                btn.disabled = false;
            }
        }
    </script>
</body>
</html>
`
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(htmlContent))
}

// 2. 新增：历史记录页面
func handleHistory(w http.ResponseWriter, r *http.Request) {
	// 简单鉴权：通过 URL 参数 token
	token := r.URL.Query().Get("token")
	if token != SecurityToken {
		http.Error(w, "🚫 无权访问：Token 错误", 403)
		return
	}

	mutex.Lock()
	records := historyList
	mutex.Unlock()

	// 倒序排列（最新的在前面）
	rows := ""
	for i := len(records) - 1; i >= 0; i-- {
		rec := records[i]
		rows += fmt.Sprintf(`
            <tr>
                <td>%s</td>
                <td class="mid">%s</td>
                <td>%s</td>
            </tr>`, rec.GenerateTime, rec.MachineID, rec.ExpiryDate)
	}

	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>生成记录</title>
    <style>
        body { font-family: -apple-system, sans-serif; max-width: 800px; margin: 40px auto; padding: 20px; background: #f5f5f7; }
        .card { background: white; padding: 20px; border-radius: 12px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
        h2 { margin-top: 0; color: #333; border-bottom: 1px solid #eee; padding-bottom: 15px; }
        table { width: 100%; border-collapse: collapse; margin-top: 10px; font-size: 14px; }
        th { text-align: left; color: #888; font-weight: 500; padding: 10px; border-bottom: 1px solid #eee; }
        td { padding: 12px 10px; border-bottom: 1px solid #f5f5f5; color: #333; }
        .mid { font-family: monospace; color: #0070f3; }
        a { display: inline-block; margin-bottom: 15px; color: #0071e3; text-decoration: none; }
    </style>
</head>
<body>
    <div class="card">
        <a href="/">← 返回生成页</a>
        <h2>📄 激活码生成记录 (%d 条)</h2>
        <table>
            <thead>
                <tr>
                    <th>生成时间</th>
                    <th>机器码</th>
                    <th>到期时间</th>
                </tr>
            </thead>
            <tbody>
                %s
            </tbody>
        </table>
    </div>
</body>
</html>
`, len(records), rows)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

// 3. API 接口
func handleAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}

	var req GenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON 错误", 400)
		return
	}

	if req.Token != SecurityToken {
		http.Error(w, "Token 错误", 403)
		return
	}

	code, err := generateLicenseCore(req.MachineID, req.Expiry)
	if err != nil {
		log.Printf("生成失败: %v", err)
		http.Error(w, err.Error(), 500)
		return
	}

	// === 记录日志 ===
	saveRecord(req.MachineID, req.Expiry)
	// ===============

	w.Write([]byte(code))
}

// ================= 核心业务逻辑 =================

func generateLicenseCore(machineID, expiryStr string) (string, error) {
	if machineID == "" || expiryStr == "" {
		return "", fmt.Errorf("缺少字段")
	}

	privKeyContent := os.Getenv("PRIVATE_KEY")
	if privKeyContent == "" {
		return "", fmt.Errorf("私钥未配置")
	}

	// 优先使用 Asia/Shanghai，失败则 UTC
	var t time.Time
	var err error
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err == nil {
		t, err = time.ParseInLocation("2006-01-02", expiryStr, loc)
	} else {
		t, err = time.Parse("2006-01-02", expiryStr)
	}
	if err != nil {
		return "", fmt.Errorf("日期错误: %v", err)
	}

	expiryUTC := t.Add(24*time.Hour - time.Second).UTC().Unix()

	dataBytes, _ := json.Marshal(LicenseData{MachineID: machineID, ExpiryUTC: expiryUTC})
	block, _ := pem.Decode([]byte(privKeyContent))
	if block == nil {
		return "", fmt.Errorf("私钥格式错误")
	}
	privKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(dataBytes)
	sig, err := rsa.SignPKCS1v15(rand.Reader, privKey, crypto.SHA256, hash[:])
	if err != nil {
		return "", err
	}

	licenseBytes, _ := json.Marshal(License{
		Data:      base64.StdEncoding.EncodeToString(dataBytes),
		Signature: base64.StdEncoding.EncodeToString(sig),
	})

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	gz.Write(licenseBytes)
	gz.Close()

	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// ================= 存储辅助函数 =================

func saveRecord(mid, expiry string) {
	mutex.Lock()
	defer mutex.Unlock()

	// 格式化当前时间 (北京时间)
	now := time.Now()
	if loc, err := time.LoadLocation("Asia/Shanghai"); err == nil {
		now = now.In(loc)
	}
	timeStr := now.Format("2006-01-02 15:04:05")

	// 添加到切片
	record := HistoryRecord{
		GenerateTime: timeStr,
		MachineID:    mid,
		ExpiryDate:   expiry,
	}
	historyList = append(historyList, record)

	// 保存到文件 (虽然云端重启会丢，但运行时不丢)
	file, _ := os.Create(historyFile)
	json.NewEncoder(file).Encode(historyList)
	file.Close()
}

func loadHistory() {
	mutex.Lock()
	defer mutex.Unlock()

	file, err := os.Open(historyFile)
	if err != nil {
		return // 文件不存在，忽略
	}
	defer file.Close()

	json.NewDecoder(file).Decode(&historyList)
	log.Printf("已加载 %d 条历史记录", len(historyList))
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}