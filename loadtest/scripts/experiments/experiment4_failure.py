#!/usr/bin/env python3
"""
Experiment 4: Failure Injection Test
测试系统容错能力，在负载测试期间手动停止ECS任务
"""

import argparse
import json
import time
import threading
import requests
import websocket
from datetime import datetime
from collections import defaultdict
import sys

class FailureStats:
    def __init__(self):
        self.lock = threading.Lock()
        self.timeline = []  # 记录时间线事件
        self.messages_sent = 0
        self.messages_succeeded = 0
        self.messages_failed = 0
        self.ws_disconnections = 0
        self.ws_reconnections = 0
        self.error_windows = []  # [(start_time, end_time, error_count)]
        self.errors = defaultdict(int)
        self.start_time = time.time()

    def log_event(self, event_type, message):
        with self.lock:
            elapsed = time.time() - self.start_time
            self.timeline.append({
                'time': elapsed,
                'type': event_type,
                'message': message
            })
            print(f"[{elapsed:.1f}s] {event_type}: {message}")

    def add_send_success(self):
        with self.lock:
            self.messages_sent += 1
            self.messages_succeeded += 1

    def add_send_failure(self, error_msg):
        with self.lock:
            self.messages_sent += 1
            self.messages_failed += 1
            self.errors[error_msg] += 1

    def add_disconnection(self):
        with self.lock:
            self.ws_disconnections += 1

    def add_reconnection(self):
        with self.lock:
            self.ws_reconnections += 1

    def get_stats(self):
        with self.lock:
            return {
                'messages_sent': self.messages_sent,
                'messages_succeeded': self.messages_succeeded,
                'messages_failed': self.messages_failed,
                'success_rate': (self.messages_succeeded / self.messages_sent * 100) if self.messages_sent > 0 else 0,
                'ws_disconnections': self.ws_disconnections,
                'ws_reconnections': self.ws_reconnections,
                'errors': dict(self.errors),
                'timeline': self.timeline.copy()
            }

class ResilientWSClient:
    """具有自动重连能力的WebSocket客户端"""
    def __init__(self, ws_url, user_id, stats):
        self.ws_url = ws_url
        self.user_id = user_id
        self.stats = stats
        self.ws = None
        self.running = False
        self.connected = False

    def start(self):
        self.running = True
        thread = threading.Thread(target=self._run)
        thread.daemon = True
        thread.start()

    def _connect(self):
        """建立WebSocket连接"""
        try:
            self.ws = websocket.create_connection(self.ws_url, timeout=10)
            auth_msg = json.dumps({"type": "auth", "user_id": self.user_id})
            self.ws.send(auth_msg)

            response = self.ws.recv()
            resp_data = json.loads(response)

            if resp_data.get('type') == 'auth_ok':
                self.connected = True
                self.stats.log_event('WS_CONNECT', f"{self.user_id} connected")
                return True

        except Exception as e:
            self.stats.log_event('WS_ERROR', f"{self.user_id} connection failed: {e}")

        return False

    def _run(self):
        """运行循环，支持自动重连"""
        while self.running:
            if not self.connected:
                # 尝试连接
                if self._connect():
                    self.stats.add_reconnection()
                else:
                    time.sleep(5)  # 连接失败，等待后重试
                    continue

            # 接收消息
            try:
                self.ws.settimeout(1.0)
                message = self.ws.recv()
                # 处理消息...

            except websocket.WebSocketTimeoutException:
                continue

            except Exception as e:
                self.stats.log_event('WS_DISCONNECT', f"{self.user_id} disconnected: {e}")
                self.stats.add_disconnection()
                self.connected = False
                if self.ws:
                    self.ws.close()
                time.sleep(1)

    def stop(self):
        self.running = False
        if self.ws:
            self.ws.close()

def send_continuous_messages(api_url, session_id, sender_id, stats, duration):
    """持续发送消息"""
    end_time = time.time() + duration

    while time.time() < end_time:
        try:
            response = requests.post(
                f"{api_url}/api/messages/send",
                json={
                    "sender_id": sender_id,
                    "session_id": session_id,
                    "content": f"msg_at_{time.time():.3f}"
                },
                timeout=10
            )

            if response.status_code == 200:
                stats.add_send_success()
            else:
                stats.add_send_failure(f"HTTP {response.status_code}")

        except Exception as e:
            stats.add_send_failure(str(e))

        time.sleep(1)  # 每秒一条消息

def run_experiment(api_url, ws_url, num_users, duration):
    """运行故障注入测试"""
    print(f"\n{'='*60}")
    print(f"Experiment 4: Failure Injection Test")
    print(f"{'='*60}")
    print(f"Users: {num_users}")
    print(f"Duration: {duration}s")
    print(f"{'='*60}\n")

    stats = FailureStats()

    # 1. 创建测试群
    print("[Setup] Creating test group...")
    member_ids = [f"failure_user_{i}" for i in range(num_users)]

    create_resp = requests.post(
        f"{api_url}/api/sessions/group",
        json={
            "creator_id": member_ids[0],
            "name": f"failure_test_{int(time.time())}",
            "member_ids": member_ids
        }
    )

    if create_resp.status_code != 200:
        print(f"✗ Failed to create group: {create_resp.text}")
        return None

    session_id = create_resp.json()['session']['session_id']
    stats.log_event('SETUP', f"Created session {session_id}")

    # 2. 启动WebSocket客户端（带自动重连）
    print(f"[Setup] Starting {num_users} resilient WebSocket clients...")
    clients = []
    for user_id in member_ids:
        client = ResilientWSClient(ws_url, user_id, stats)
        client.start()
        clients.append(client)
        time.sleep(0.01)

    time.sleep(3)
    stats.log_event('SETUP', 'All clients started')

    # 3. 启动消息发送线程
    print(f"[Test] Starting message sender...")
    sender_thread = threading.Thread(
        target=send_continuous_messages,
        args=(api_url, session_id, member_ids[0], stats, duration)
    )
    sender_thread.start()

    stats.log_event('TEST_START', 'Continuous message sending started')

    # 4. 提示用户手动注入故障
    print(f"\n{'='*60}")
    print(f"MANUAL ACTION REQUIRED")
    print(f"{'='*60}")
    print(f"\n在另一个终端执行以下命令来停止一个ECS任务：\n")
    print(f"# 1. 获取任务列表")
    print(f"aws ecs list-tasks \\")
    print(f"  --cluster chatroom-dev-cluster \\")
    print(f"  --service-name chatroom-dev-gateway \\")
    print(f"  --region us-west-2\n")
    print(f"# 2. 停止一个任务（复制上面命令输出的某个任务ARN）")
    print(f"aws ecs stop-task \\")
    print(f"  --cluster chatroom-dev-cluster \\")
    print(f"  --task <TASK_ARN> \\")
    print(f"  --region us-west-2\n")
    print(f"测试将运行 {duration} 秒")
    print(f"建议在测试开始后30-60秒时注入故障")
    print(f"{'='*60}\n")

    # 5. 等待测试完成，期间持续监控
    start = time.time()
    last_report = start

    while time.time() - start < duration:
        time.sleep(5)

        # 每10秒报告一次状态
        if time.time() - last_report > 10:
            current_stats = stats.get_stats()
            print(f"\n[Status Report]")
            print(f"  Elapsed: {time.time() - start:.0f}s / {duration}s")
            print(f"  Messages: {current_stats['messages_succeeded']}/{current_stats['messages_sent']} succeeded")
            print(f"  WS Disconnections: {current_stats['ws_disconnections']}")
            print(f"  WS Reconnections: {current_stats['ws_reconnections']}")
            last_report = time.time()

    # 6. 等待发送线程完成
    sender_thread.join()

    # 7. 停止所有客户端
    print(f"\n[Cleanup] Stopping clients...")
    for client in clients:
        client.stop()

    stats.log_event('TEST_END', 'Test completed')

    # 8. 统计结果
    final_stats = stats.get_stats()

    print(f"\n{'='*60}")
    print(f"RESULTS")
    print(f"{'='*60}")
    print(f"Duration: {duration}s")
    print(f"Messages sent: {final_stats['messages_sent']}")
    print(f"Messages succeeded: {final_stats['messages_succeeded']}")
    print(f"Messages failed: {final_stats['messages_failed']}")
    print(f"Success rate: {final_stats['success_rate']:.2f}%")
    print(f"WebSocket disconnections: {final_stats['ws_disconnections']}")
    print(f"WebSocket reconnections: {final_stats['ws_reconnections']}")

    if final_stats['errors']:
        print(f"\nError breakdown:")
        for error, count in final_stats['errors'].items():
            print(f"  - {error}: {count}")

    print(f"\n事件时间线：")
    for event in final_stats['timeline'][:20]:  # 显示前20个事件
        print(f"  [{event['time']:.1f}s] {event['type']}: {event['message']}")
    if len(final_stats['timeline']) > 20:
        print(f"  ... (总共 {len(final_stats['timeline'])} 个事件)")

    print(f"{'='*60}\n")

    # 分析
    print("分析：")
    if final_stats['ws_disconnections'] > 0:
        print(f"✓ 检测到 {final_stats['ws_disconnections']} 次WebSocket断连")
        if final_stats['ws_reconnections'] > 0:
            print(f"✓ 成功重连 {final_stats['ws_reconnections']} 次")
        else:
            print(f"✗ 没有成功重连")
    else:
        print(f"✗ 没有检测到断连（可能没有注入故障？）")

    if final_stats['success_rate'] < 95:
        print(f"⚠ 消息成功率较低 ({final_stats['success_rate']:.1f}%)")
    else:
        print(f"✓ 消息成功率良好 ({final_stats['success_rate']:.1f}%)")

    print()

    # 保存结果
    result = {
        'experiment': 'failure_injection',
        'timestamp': datetime.now().isoformat(),
        'config': {
            'num_users': num_users,
            'duration': duration
        },
        'results': final_stats
    }

    filename = f"experiment4_results_{num_users}users_{duration}s_{datetime.now().strftime('%Y%m%d_%H%M%S')}.json"
    with open(filename, 'w') as f:
        json.dump(result, f, indent=2)

    print(f"Results saved to: {filename}\n")

    return result

def main():
    parser = argparse.ArgumentParser(description='Experiment 4: Failure Injection Test')
    parser.add_argument('--users', type=int, default=100,
                        help='Number of users (default: 100)')
    parser.add_argument('--duration', type=int, default=300,
                        help='Test duration in seconds (default: 300)')
    parser.add_argument('--config', type=str, default='config.json',
                        help='Config file path (default: config.json)')

    args = parser.parse_args()

    # 加载配置
    try:
        with open(args.config, 'r') as f:
            config = json.load(f)
    except Exception as e:
        print(f"Error loading config: {e}")
        sys.exit(1)

    api_url = config['alb_url']
    ws_url = config['ws_url']

    # 运行测试
    run_experiment(
        api_url=api_url,
        ws_url=ws_url,
        num_users=args.users,
        duration=args.duration
    )

if __name__ == '__main__':
    main()
