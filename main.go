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
	"time"
)

// ================= 全局配置 =================

// SecurityToken 从环境变量获取验证 Token，防止未授权访问
// 如果未设置环境变量 SECURITY_TOKEN，默认值为 "123456"
var SecurityToken = getEnv("SECURITY_TOKEN", "123456")

// ================= 数据结构定义 =================

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
	Expiry    string `json:"expiry"` // 格式 YYYY-MM-DD
}

// ================= 主程序入口 =================

func main() {
	// 1. 检查私钥环境变量（仅做日志提示，不阻塞启动）
	if os.Getenv("PRIVATE_KEY") == "" {
		log.Println("⚠️  警告: 环境变量 PRIVATE_KEY 未设置！")
		log.Println("请在云平台设置 PRIVATE_KEY (私钥内容) 和 SECURITY_TOKEN (访问密码)。")
	} else {
		log.Println("✅ 检测到私钥配置，服务准备就绪。")
	}

	// 2. 注册 HTTP 路由处理函数
	http.HandleFunc("/", handleIndex)           // 网页界面
	http.HandleFunc("/api/generate", handleAPI) // 生成接口
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("OK"))
	})

	// 3. 获取端口并启动服务
	port := getEnv("PORT", "8080")
	log.Printf("🚀 服务已启动，监听端口 :%s", port)

	// 启动监听 (这一步是阻塞的，必须放在 main 函数的最后)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal("服务启动失败:", err)
	}
}

// ================= HTTP 处理函数 =================

// handleIndex 返回内嵌的 HTML 前端页面
func handleIndex(w http.ResponseWriter, r *http.Request) {
	// 使用反引号 ` 定义多行字符串
	htmlContent := `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>激活码生成后台</title>
    <style>
        body { font-family: -apple-system, sans-serif; max-width: 600px; margin: 40px auto; padding: 20px; background: #f5f5f7; color: #333; }
        .card { background: white; padding: 30px; border-radius: 12px; box-shadow: 0 4px 12px rgba(0,0,0,0.1); }
        h2 { margin-top: 0; color: #0071e3; }
        .form-group { margin-bottom: 20px; }
        label { display: block; margin-bottom: 8px; font-weight: 600; font-size: 14px; }
        input { width: 100%; padding: 12px; border: 1px solid #d2d2d7; border-radius: 8px; font-size: 16px; box-sizing: border-box; }
        button { width: 100%; padding: 14px; background: #0071e3; color: white; border: none; border-radius: 8px; font-size: 16px; font-weight: 600; cursor: pointer; transition: background 0.2s; }
        button:hover { background: #0077ed; }
        button:disabled { background: #ccc; cursor: not-allowed; }
        #result { margin-top: 25px; padding: 15px; background: #1d1d1f; color: #fff; border-radius: 8px; font-family: monospace; word-break: break-all; display: none; line-height: 1.5; }
        .error { background: #ffe5e5 !important; color: #d70015 !important; border: 1px solid #ff3b30; }
    </style>
</head>
<body>
    <div class="card">
        <h2>🔐 激活码生成器</h2>

        <div class="form-group">
            <label>鉴权密码 (SECURITY_TOKEN)</label>
            <input type="password" id="token" placeholder="输入云端设置的 Token">
        </div>

        <div class="form-group">
            <label>客户机器码 (Machine ID)</label>
            <input type="text" id="mid" placeholder="粘贴客户提供的机器码">
        </div>

        <div class="form-group">
            <label>到期日期</label>
            <input type="date" id="date">
        </div>

        <button onclick="generate()" id="btn">生成激活码</button>
        <div id="result"></div>
    </div>

    <script>
        // 初始化日期为明天
        const tomorrow = new Date();
        tomorrow.setDate(tomorrow.getDate() + 1);
        document.getElementById('date').valueAsDate = tomorrow;

        async function generate() {
            const resDiv = document.getElementById('result');
            const btn = document.getElementById('btn');

            // UI 状态重置
            resDiv.style.display = 'block';
            resDiv.innerText = "正在计算签名...";
            resDiv.className = '';
            btn.disabled = true;
            btn.innerText = "生成中...";

            const payload = {
                token: document.getElementById('token').value,
                machine_id: document.getElementById('mid').value,
                expiry: document.getElementById('date').value
            };

            try {
                const response = await fetch('/api/generate', {
                    method: 'POST',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify(payload)
                });

                const text = await response.text();

                if (response.ok) {
                    resDiv.innerText = text; // 显示成功生成的激活码
                } else {
                    resDiv.innerText = "❌ 错误: " + text;
                    resDiv.className = 'error';
                }
            } catch (err) {
                resDiv.innerText = "❌ 网络请求失败: " + err;
                resDiv.className = 'error';
            } finally {
                btn.disabled = false;
                btn.innerText = "生成激活码";
            }
        }
    </script>
</body>
</html>
`
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(htmlContent))
}

// handleAPI 处理生成激活码的 API 请求
func handleAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req GenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON 格式错误", http.StatusBadRequest)
		return
	}

	// 鉴权检查
	if req.Token != SecurityToken {
		log.Printf("鉴权失败: 收到 token=%s, 期望 token=%s", req.Token, SecurityToken)
		http.Error(w, "🚫 鉴权失败: Token 错误", http.StatusForbidden)
		return
	}

	// 调用核心生成逻辑
	code, err := generateLicenseCore(req.MachineID, req.Expiry)
	if err != nil {
		log.Printf("生成失败: %v", err)
		http.Error(w, fmt.Sprintf("生成失败: %v", err), http.StatusInternalServerError)
		return
	}

	w.Write([]byte(code))
}

// ================= 核心业务逻辑 =================

func generateLicenseCore(machineID, expiryStr string) (string, error) {
	if machineID == "" || expiryStr == "" {
		return "", fmt.Errorf("机器码或日期不能为空")
	}

	// 1. 获取私钥内容
	privKeyContent := os.Getenv("PRIVATE_KEY")
	if privKeyContent == "" {
		return "", fmt.Errorf("服务器端未配置私钥 (环境变量 PRIVATE_KEY 为空)")
	}

	// 2. 解析日期 (优先使用 Asia/Shanghai，失败则回退到 UTC)
	var t time.Time
	var err error
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err == nil {
		t, err = time.ParseInLocation("2006-01-02", expiryStr, loc)
	} else {
		t, err = time.Parse("2006-01-02", expiryStr)
	}

	if err != nil {
		return "", fmt.Errorf("日期格式错误: %v", err)
	}

	// 设置到期时间为当天的 23:59:59
	expiryUTC := t.Add(24*time.Hour - time.Second).UTC().Unix()

	// 3. 构建数据 Payload
	dataBytes, _ := json.Marshal(LicenseData{
		MachineID: machineID,
		ExpiryUTC: expiryUTC,
	})

	// 4. 解析 PEM 私钥
	block, _ := pem.Decode([]byte(privKeyContent))
	if block == nil {
		return "", fmt.Errorf("私钥格式解析失败 (不是有效的 PEM 格式)")
	}

	privKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("解析 RSA 私钥失败: %v", err)
	}

	// 5. 进行 SHA256 签名
	hash := sha256.Sum256(dataBytes)
	sig, err := rsa.SignPKCS1v15(rand.Reader, privKey, crypto.SHA256, hash[:])
	if err != nil {
		return "", fmt.Errorf("签名过程失败: %v", err)
	}

	// 6. 组合最终 License 结构
	licenseBytes, _ := json.Marshal(License{
		Data:      base64.StdEncoding.EncodeToString(dataBytes),
		Signature: base64.StdEncoding.EncodeToString(sig),
	})

	// 7. Gzip 压缩 + Base64 编码
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(licenseBytes); err != nil {
		return "", err
	}
	if err := gz.Close(); err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// ================= 辅助函数 =================

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}