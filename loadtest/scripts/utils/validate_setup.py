#!/usr/bin/env python3
"""
验证负载测试环境配置
"""

import sys
import json
import requests
import websocket

def check_config():
    """检查配置文件"""
    print("1. 检查配置文件...")
    try:
        with open('config.json', 'r') as f:
            config = json.load(f)

        required_keys = ['alb_url', 'ws_url', 'region', 'cluster_name']
        for key in required_keys:
            if key not in config:
                print(f"   ✗ 缺少配置项: {key}")
                return False

        print(f"   ✓ 配置文件正常")
        print(f"     ALB URL: {config['alb_url']}")
        print(f"     WS URL: {config['ws_url']}")
        return config

    except Exception as e:
        print(f"   ✗ 读取配置文件失败: {e}")
        return False

def check_http_health(alb_url):
    """检查HTTP健康状态"""
    print("\n2. 检查HTTP健康状态...")
    try:
        response = requests.get(f"{alb_url}/health", timeout=10)
        if response.status_code == 200:
            data = response.json()
            if data.get('status') == 'ok':
                print(f"   ✓ HTTP健康检查通过")
                return True
            else:
                print(f"   ✗ 健康状态异常: {data}")
                return False
        else:
            print(f"   ✗ HTTP状态码: {response.status_code}")
            return False

    except Exception as e:
        print(f"   ✗ HTTP请求失败: {e}")
        return False

def check_websocket(ws_url):
    """检查WebSocket连接"""
    print("\n3. 检查WebSocket连接...")
    try:
        ws = websocket.create_connection(ws_url, timeout=10)

        # 发送认证
        auth_msg = json.dumps({
            "type": "auth",
            "user_id": "test_user_validation"
        })
        ws.send(auth_msg)

        # 接收响应
        response = ws.recv()
        data = json.loads(response)

        ws.close()

        if data.get('type') == 'auth_ok':
            print(f"   ✓ WebSocket连接成功")
            return True
        else:
            print(f"   ✗ WebSocket认证失败: {data}")
            return False

    except Exception as e:
        print(f"   ✗ WebSocket连接失败: {e}")
        return False

def check_create_session(alb_url):
    """检查创建会话"""
    print("\n4. 检查创建群聊...")
    try:
        response = requests.post(
            f"{alb_url}/api/sessions/group",
            json={
                "creator_id": "validation_user_1",
                "name": "validation_test_group",
                "member_ids": ["validation_user_1", "validation_user_2"]
            },
            timeout=10
        )

        if response.status_code == 200:
            data = response.json()
            session_id = data.get('session', {}).get('session_id')
            print(f"   ✓ 创建群聊成功")
            print(f"     Session ID: {session_id}")
            return session_id
        else:
            print(f"   ✗ 创建群聊失败: HTTP {response.status_code}")
            print(f"     {response.text}")
            return None

    except Exception as e:
        print(f"   ✗ 创建群聊失败: {e}")
        return None

def check_send_message(alb_url, session_id):
    """检查发送消息"""
    print("\n5. 检查发送消息...")
    try:
        response = requests.post(
            f"{alb_url}/api/messages/send",
            json={
                "sender_id": "validation_user_1",
                "session_id": session_id,
                "content": "validation test message"
            },
            timeout=10
        )

        if response.status_code == 200:
            data = response.json()
            msg_id = data.get('message', {}).get('message_id')
            print(f"   ✓ 发送消息成功")
            print(f"     Message ID: {msg_id}")
            return True
        else:
            print(f"   ✗ 发送消息失败: HTTP {response.status_code}")
            print(f"     {response.text}")
            return False

    except Exception as e:
        print(f"   ✗ 发送消息失败: {e}")
        return False

def check_dependencies():
    """检查Python依赖"""
    print("\n6. 检查Python依赖...")
    required = ['requests', 'websocket', 'numpy', 'pandas']
    missing = []

    for module in required:
        try:
            if module == 'websocket':
                __import__('websocket')
            else:
                __import__(module)
            print(f"   ✓ {module}")
        except ImportError:
            print(f"   ✗ {module} (未安装)")
            missing.append(module)

    if missing:
        print(f"\n   请安装缺失的依赖:")
        print(f"   pip install {' '.join(missing)}")
        return False

    return True

def main():
    print("="*60)
    print("  负载测试环境验证")
    print("="*60)

    all_passed = True

    # 检查依赖
    if not check_dependencies():
        all_passed = False

    # 检查配置
    config = check_config()
    if not config:
        all_passed = False
        print("\n请创建 config.json 文件并填入正确的配置")
        sys.exit(1)

    alb_url = config['alb_url']
    ws_url = config['ws_url']

    # 检查HTTP
    if not check_http_health(alb_url):
        all_passed = False

    # 检查WebSocket
    if not check_websocket(ws_url):
        all_passed = False

    # 检查创建会话
    session_id = check_create_session(alb_url)
    if not session_id:
        all_passed = False

    # 检查发送消息
    if session_id:
        if not check_send_message(alb_url, session_id):
            all_passed = False

    # 总结
    print("\n" + "="*60)
    if all_passed:
        print("  ✓ 所有检查通过！")
        print("="*60)
        print("\n可以开始运行负载测试了:")
        print("  python experiment1_connection_scaling.py --connections 100")
        print("  或者运行: ./run_all_tests.sh")
        sys.exit(0)
    else:
        print("  ✗ 部分检查失败")
        print("="*60)
        print("\n请修复上述问题后再运行负载测试")
        sys.exit(1)

if __name__ == '__main__':
    main()
