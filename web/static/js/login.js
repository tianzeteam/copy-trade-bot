// 页面加载完成后执行
document.addEventListener('DOMContentLoaded', function() {
    // 检查是否已登录
    const authToken = localStorage.getItem('auth_token');
    if (authToken) {
        // 验证令牌有效性
        verifyToken(authToken)
            .then(valid => {
                if (valid) {
                    // 令牌有效，重定向到仪表盘
                    window.location.href = '/dashboard';
                } else {
                    // 令牌无效，清除本地存储
                    localStorage.removeItem('auth_token');
                    localStorage.removeItem('username');
                }
            })
            .catch(error => {
                console.error('验证令牌失败:', error);
                // 清除本地存储
                localStorage.removeItem('auth_token');
                localStorage.removeItem('username');
            });
    }

    // 注册登录表单提交事件
    document.getElementById('loginForm').addEventListener('submit', function(e) {
        e.preventDefault();
        
        // 获取表单数据
        const username = document.getElementById('username').value;
        const password = document.getElementById('password').value;
        const rememberMe = document.getElementById('rememberMe').checked;
        
        // 发送登录请求
        login(username, password, rememberMe);
    });
});

// 登录函数
function login(username, password, rememberMe) {
    // 隐藏错误提示
    hideError();
    
    // 显示加载状态
    document.getElementById('loginButton').disabled = true;
    document.getElementById('loginButton').innerHTML = '<span class="spinner-border spinner-border-sm" role="status" aria-hidden="true"></span> 登录中...';
    
    // 发送登录请求
    fetch('/api/auth/login', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify({
            username: username,
            password: password,
            remember_me: rememberMe
        })
    })
    .then(response => {
        // 恢复按钮状态
        document.getElementById('loginButton').disabled = false;
        document.getElementById('loginButton').textContent = '登录';
        
        if (!response.ok) {
            throw new Error('登录失败');
        }
        return response.json();
    })
    .then(data => {
        // 保存令牌到本地存储
        localStorage.setItem('auth_token', data.token);
        localStorage.setItem('username', data.username);
        
        // 重定向到仪表盘
        window.location.href = '/dashboard';
    })
    .catch(error => {
        console.error('登录失败:', error);
        showError('用户名或密码错误，请重试');
    });
}

// 验证令牌
function verifyToken(token) {
    return fetch('/api/auth/verify', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${token}`
        }
    })
    .then(response => {
        if (!response.ok) {
            return false;
        }
        return response.json();
    })
    .then(data => {
        return data.valid === true;
    })
    .catch(() => {
        return false;
    });
}

// 显示错误信息
function showError(message) {
    const errorAlert = document.getElementById('errorAlert');
    errorAlert.textContent = message;
    errorAlert.classList.remove('d-none');
}

// 隐藏错误信息
function hideError() {
    document.getElementById('errorAlert').classList.add('d-none');
}
