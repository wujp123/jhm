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

type HistoryRecord struct {
	GenerateTime string `json:"generate_time"`
	MachineID    string `json:"machine_id"`
	ExpiryDate   string `json:"expiry_date"`
	LicenseCode  string `json:"license_code"`
}

// ================= 全局存储 =================

var (
	historyList []HistoryRecord
	historyFile = "history.json"
	mutex       sync.Mutex
)

// ================= 主程序入口 =================

func main() {
	loadHistory()

	if os.Getenv("PRIVATE_KEY") == "" {
		log.Println("⚠️  警告: 环境变量 PRIVATE_KEY 未设置！")
	}

	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/history", handleHistory)
	http.HandleFunc("/api/generate", handleAPI)
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("OK"))
	})

	port := getEnv("PORT", "80")
	log.Printf("🚀 服务已启动，监听端口 :%s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}

// ================= HTTP 处理函数 =================

func handleIndex(w http.ResponseWriter, r *http.Request) {
	htmlContent := `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>激活码生成器</title>
    <style>
        body { font-family: -apple-system, sans-serif; max-width: 600px; margin: 20px auto; padding: 20px; background: #f5f5f7; color: #333; }
        .card { background: white; padding: 30px; border-radius: 12px; box-shadow: 0 4px 12px rgba(0,0,0,0.1); }
        .header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 25px; border-bottom: 1px solid #eee; padding-bottom: 15px; }
        h2 { margin: 0; color: #0071e3; font-size: 22px; }
        .history-btn { font-size: 14px; color: #0071e3; text-decoration: none; font-weight: 600; padding: 6px 12px; background: #eef6ff; border-radius: 6px; transition: all 0.2s; }
        .history-btn:hover { background: #dcebff; }
        .form-group { margin-bottom: 15px; }
        label { display: block; margin-bottom: 6px; font-weight: 600; font-size: 14px; }
        input { width: 100%; padding: 12px; border: 1px solid #d2d2d7; border-radius: 8px; font-size: 16px; box-sizing: border-box; }
        button { width: 100%; padding: 14px; background: #0071e3; color: white; border: none; border-radius: 8px; font-size: 16px; font-weight: 600; cursor: pointer; transition: background 0.2s; margin-top: 10px; }
        button:hover { background: #0077ed; }
        button:disabled { background: #ccc; cursor: not-allowed; }

        #result-container { display: none; margin-top: 25px; }
        .result-label { font-size: 12px; color: #888; margin-bottom: 5px; display: flex; justify-content: space-between; }
        #result {
            padding: 15px;
            background: #1d1d1f;
            color: #fff;
            border-radius: 8px;
            font-family: monospace;
            word-break: break-all;
            line-height: 1.5;
            cursor: pointer;
            position: relative;
            transition: background 0.2s;
        }
        #result:hover { background: #333; }
        #result:active { transform: scale(0.99); }
        .copy-hint { font-size: 12px; color: #aaa; }

        .toast { position: fixed; bottom: 20px; left: 50%; transform: translateX(-50%); background: rgba(0,0,0,0.8); color: white; padding: 10px 20px; border-radius: 20px; font-size: 14px; opacity: 0; transition: opacity 0.3s; pointer-events: none; }
        .toast.show { opacity: 1; }
        .error { background: #ffe5e5 !important; color: #d70015 !important; border: 1px solid #ff3b30; cursor: default !important; }
    </style>
</head>
<body>
    <div class="card">
        <div class="header">
            <h2>🔐 激活码生成</h2>
            <a href="#" onclick="goToHistory(); return false;" class="history-btn">📄 历史记录</a>
        </div>

        <div class="form-group">
            <label>鉴权密码 (Token)</label>
            <input type="password" id="token" placeholder="输入部署时设置的密码">
        </div>
        <div class="form-group">
            <label>客户机器码 (Machine ID)</label>
            <input type="text" id="mid" placeholder="粘贴客户机器码">
        </div>
        <div class="form-group">
            <label>到期日期 (最长1个月)</label>
            <input type="date" id="date">
        </div>
        <button onclick="generate()" id="btn">生成激活码</button>

        <div id="result-container">
            <div class="result-label">
                <span>生成结果：</span>
                <span class="copy-hint">📋 点击下方黑色区域即可复制</span>
            </div>
            <div id="result" onclick="copyResult()"></div>
        </div>
    </div>

    <div id="toast" class="toast">已复制到剪贴板 ✅</div>

    <script>
        const tomorrow = new Date();
        tomorrow.setDate(tomorrow.getDate() + 1);
        document.getElementById('date').valueAsDate = tomorrow;
        const savedToken = localStorage.getItem('license_token');
        if(savedToken) document.getElementById('token').value = savedToken;

        function goToHistory() {
            const t = document.getElementById('token').value;
            const finalToken = t || localStorage.getItem('license_token');
            if(!finalToken) {
                alert('请先在输入框填入【鉴权密码】！');
                document.getElementById('token').focus();
                return;
            }
            window.location.href = '/history?token=' + finalToken;
        }

        async function generate() {
            const container = document.getElementById('result-container');
            const resDiv = document.getElementById('result');
            const btn = document.getElementById('btn');
            const token = document.getElementById('token').value;
            localStorage.setItem('license_token', token);
            container.style.display = 'block';
            resDiv.innerText = "生成中...";
            resDiv.className = '';
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
                    resDiv.onclick = copyResult;
                } else {
                    resDiv.innerText = "❌ 错误: " + text;
                    resDiv.className = 'error';
                    resDiv.onclick = null;
                }
            } catch (err) {
                resDiv.innerText = "❌ 网络请求失败: " + err;
                resDiv.className = 'error';
                resDiv.onclick = null;
            } finally {
                btn.disabled = false;
            }
        }

        function copyResult() {
            const text = document.getElementById('result').innerText;
            if (!text || text.startsWith("生成中") || text.startsWith("❌")) return;
            navigator.clipboard.writeText(text).then(() => {
                showToast("已复制激活码 ✅");
            }).catch(() => {
                alert("复制失败，请手动复制");
            });
        }

        function showToast(msg) {
            const toast = document.getElementById('toast');
            toast.innerText = msg;
            toast.classList.add('show');
            setTimeout(() => toast.classList.remove('show'), 2000);
        }
    </script>
</body>
</html>
`
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(htmlContent))
}

func handleHistory(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token != SecurityToken {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(403)
		w.Write([]byte(`<h1>🚫 访问拒绝</h1><p>Token 错误。<a href="/">返回首页</a></p>`))
		return
	}

	mutex.Lock()
	records := historyList
	mutex.Unlock()

	rows := ""
	for i := len(records) - 1; i >= 0; i-- {
		rec := records[i]

		shortCode := rec.LicenseCode
		if len(shortCode) > 12 {
			shortCode = shortCode[:12] + "..."
		}
		if shortCode == "" {
			shortCode = "(无数据)"
		}

		rows += fmt.Sprintf(`
            <tr>
                <td>%s</td>
                <td class="mid">%s</td>
                <td>%s</td>
                <td class="code-col">
                    <span class="code-preview">%s</span>
                    <button class="copy-btn" onclick="copyText('%s')">复制</button>
                </td>
            </tr>`,
            rec.GenerateTime,
            rec.MachineID,
            rec.ExpiryDate,
            shortCode,
            rec.LicenseCode,
        )
	}

	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>生成记录</title>
    <style>
        body { font-family: -apple-system, sans-serif; max-width: 900px; margin: 40px auto; padding: 20px; background: #f5f5f7; }
        .card { background: white; padding: 20px; border-radius: 12px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
        .header { display: flex; align-items: center; border-bottom: 1px solid #eee; padding-bottom: 15px; margin-bottom: 10px; }
        h2 { margin: 0; color: #333; flex-grow: 1; text-align: center; }
        .back-btn { color: #0071e3; text-decoration: none; font-weight: bold; }

        table { width: 100%%; border-collapse: collapse; margin-top: 10px; font-size: 14px; table-layout: fixed; }
        th { text-align: left; color: #888; font-weight: 500; padding: 10px; border-bottom: 1px solid #eee; white-space: nowrap; }
        td { padding: 12px 10px; border-bottom: 1px solid #f5f5f5; color: #333; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

        .mid { font-family: monospace; color: #0070f3; }
        .code-col { display: flex; align-items: center; justify-content: space-between; }
        .code-preview { font-family: monospace; color: #666; background: #eee; padding: 2px 6px; border-radius: 4px; font-size: 12px; }

        .copy-btn {
            background: white; border: 1px solid #d2d2d7; color: #333;
            padding: 4px 10px; border-radius: 4px; cursor: pointer; font-size: 12px;
            margin-left: 8px; transition: all 0.2s;
        }
        .copy-btn:hover { background: #f5f5f7; border-color: #999; }
        .copy-btn:active { background: #e5e5e5; }

        tr:hover { background-color: #f9f9fa; }

        .toast { position: fixed; bottom: 20px; left: 50%%; transform: translateX(-50%%); background: rgba(0,0,0,0.8); color: white; padding: 10px 20px; border-radius: 20px; font-size: 14px; opacity: 0; transition: opacity 0.3s; pointer-events: none; z-index: 999; }
        .toast.show { opacity: 1; }

        @media (max-width: 600px) {
            th:nth-child(1), td:nth-child(1) { width: 80px; font-size: 12px; }
            th:nth-child(2), td:nth-child(2) { display: none; }
            th:nth-child(3), td:nth-child(3) { width: 90px; }
        }
    </style>
</head>
<body>
    <div class="card">
        <div class="header">
            <a href="/" class="back-btn">← 返回</a>
            <h2>📄 激活码生成记录 (%d 条)</h2>
            <div style="width: 50px;"></div>
        </div>
        <table>
            <thead>
                <tr>
                    <th style="width: 150px;">生成时间</th>
                    <th>机器码</th>
                    <th style="width: 100px;">到期时间</th>
                    <th style="width: 160px;">激活码</th>
                </tr>
            </thead>
            <tbody>
                %s
            </tbody>
        </table>
    </div>

    <div id="toast" class="toast">已复制 ✅</div>

    <script>
        function copyText(text) {
            if (!text) return;
            navigator.clipboard.writeText(text).then(() => {
                const toast = document.getElementById('toast');
                toast.classList.add('show');
                setTimeout(() => toast.classList.remove('show'), 2000);
            }).catch(err => {
                alert('复制失败');
                console.error(err);
            });
        }
    </script>
</body>
</html>
`, len(records), rows)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

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
		http.Error(w, "鉴权失败", 403)
		return
	}

	code, err := generateLicenseCore(req.MachineID, req.Expiry)
	if err != nil {
		log.Printf("生成失败: %v", err)
		http.Error(w, err.Error(), 500)
		return
	}

	saveRecord(req.MachineID, req.Expiry, code)
	w.Write([]byte(code))
}

// ================= 核心逻辑 =================

func generateLicenseCore(machineID, expiryStr string) (string, error) {
	if machineID == "" || expiryStr == "" {
		return "", fmt.Errorf("字段为空")
	}
	privKeyContent := os.Getenv("PRIVATE_KEY")
	if privKeyContent == "" {
		return "", fmt.Errorf("私钥未配置")
	}

	var t time.Time
	var err error
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err == nil {
		t, err = time.ParseInLocation("2006-01-02", expiryStr, loc)
	} else {
		t, err = time.Parse("2006-01-02", expiryStr)
	}
	if err != nil {
		return "", err
	}

	// ==========================================
	// 🔥 核心修改：增加 1 个月期限限制校验
	// ==========================================
	now := time.Now().In(loc)
	// 计算最大允许日期：当前时间 + 1 个月
	maxAllowed := now.AddDate(0, 1, 0)

	// t 是用户选中日期的 00:00:00
	// 如果选中的日期 (t) 晚于当前时间往后推一个月 (maxAllowed)，则报错
	if t.After(maxAllowed) {
		return "", fmt.Errorf("生成失败：有效期不能超过 1 个月\n当前最晚允许: %s", maxAllowed.Format("2006-01-02"))
	}
	// ==========================================

	expiryUTC := t.Add(24*time.Hour - time.Second).UTC().Unix()

	dataBytes, _ := json.Marshal(LicenseData{MachineID: machineID, ExpiryUTC: expiryUTC})
	block, _ := pem.Decode([]byte(privKeyContent))
	if block == nil {
		return "", fmt.Errorf("私钥错误")
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

// ================= 存储 =================

func saveRecord(mid, expiry, code string) {
	mutex.Lock()
	defer mutex.Unlock()
	now := time.Now()
	if loc, err := time.LoadLocation("Asia/Shanghai"); err == nil {
		now = now.In(loc)
	}
	historyList = append(historyList, HistoryRecord{
		GenerateTime: now.Format("2006-01-02 15:04:05"),
		MachineID:    mid,
		ExpiryDate:   expiry,
		LicenseCode:  code,
	})
	file, _ := os.Create(historyFile)
	json.NewEncoder(file).Encode(historyList)
	file.Close()
}

func loadHistory() {
	mutex.Lock()
	defer mutex.Unlock()
	file, err := os.Open(historyFile)
	if err == nil {
		json.NewDecoder(file).Decode(&historyList)
		file.Close()
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}