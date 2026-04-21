#!/usr/bin/env python3
"""
Experiment 2: Throughput Scaling Test
固定用户数，测试不同消息速率下的系统性能
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
import statistics

class ThroughputStats:
    def __init__(self):
        self.lock = threading.Lock()
        self.messages_sent = 0
        self.messages_received = 0
        self.send_latencies = []
        self.e2e_latencies = []  # end-to-end latency
        self.errors = defaultdict(int)
        self.error_rate_samples = []

    def add_send(self, latency):
        with self.lock:
            self.messages_sent += 1
            self.send_latencies.append(latency)

    def add_receive(self, e2e_latency):
        with self.lock:
            self.messages_received += 1
            self.e2e_latencies.append(e2e_latency)

    def add_error(self, error_msg):
        with self.lock:
            self.errors[error_msg] += 1

    def get_stats(self):
        with self.lock:
            stats = {
                'messages_sent': self.messages_sent,
                'messages_received': self.messages_received,
                'errors': dict(self.errors),
                'total_errors': sum(self.errors.values())
            }

            if self.send_latencies:
                stats['send_latency'] = {
                    'avg': statistics.mean(self.send_latencies),
                    'median': statistics.median(self.send_latencies),
                    'p95': self._percentile(self.send_latencies, 95),
                    'p99': self._percentile(self.send_latencies, 99),
                    'max': max(self.send_latencies),
                    'min': min(self.send_latencies)
                }

            if self.e2e_latencies:
                stats['e2e_latency'] = {
                    'avg': statistics.mean(self.e2e_latencies),
                    'median': statistics.median(self.e2e_latencies),
                    'p95': self._percentile(self.e2e_latencies, 95),
                    'p99': self._percentile(self.e2e_latencies, 99),
                    'max': max(self.e2e_latencies),
                    'min': min(self.e2e_latencies)
                }

            if self.messages_sent > 0:
                stats['error_rate'] = (sum(self.errors.values()) / (self.messages_sent + sum(self.errors.values()))) * 100

            return stats

    def _percentile(self, data, percentile):
        sorted_data = sorted(data)
        index = int(len(sorted_data) * percentile / 100)
        return sorted_data[min(index, len(sorted_data) - 1)]

class WSReceiver:
    """WebSocket接收器，监听收到的消息"""
    def __init__(self, ws_url, user_id, stats):
        self.ws_url = ws_url
        self.user_id = user_id
        self.stats = stats
        self.ws = None
        self.running = False
        self.message_timestamps = {}  # message_id -> send_time

    def start(self):
        """启动接收器"""
        self.running = True
        thread = threading.Thread(target=self._run)
        thread.daemon = True
        thread.start()
        time.sleep(0.5)  # 等待连接建立

    def _run(self):
        """运行接收循环"""
        try:
            self.ws = websocket.create_connection(self.ws_url, timeout=10)

            # 认证
            auth_msg = json.dumps({"type": "auth", "user_id": self.user_id})
            self.ws.send(auth_msg)

            # 接收认证响应
            response = self.ws.recv()
            resp_data = json.loads(response)

            if resp_data.get('type') != 'auth_ok':
                print(f"✗ {self.user_id} auth failed")
                return

            print(f"✓ {self.user_id} receiver connected")

            # 接收消息循环
            while self.running:
                try:
                    self.ws.settimeout(1.0)
                    message = self.ws.recv()
                    data = json.loads(message)

                    if data.get('type') == 'new_message':
                        msg = data.get('message', {})
                        msg_id = msg.get('message_id')

                        # 计算端到端延迟
                        if msg_id in self.message_timestamps:
                            send_time = self.message_timestamps[msg_id]
                            e2e_latency = (time.time() - send_time) * 1000  # ms
                            self.stats.add_receive(e2e_latency)

                except websocket.WebSocketTimeoutException:
                    continue
                except Exception as e:
                    if self.running:
                        print(f"✗ Receiver error: {e}")
                    break

        except Exception as e:
            print(f"✗ Receiver connection error: {e}")
        finally:
            if self.ws:
                self.ws.close()

    def register_message(self, msg_id):
        """注册消息发送时间"""
        self.message_timestamps[msg_id] = time.time()

    def stop(self):
        """停止接收器"""
        self.running = False

def send_message(api_url, sender_id, session_id, content, stats):
    """发送一条消息并记录延迟"""
    start_time = time.time()

    try:
        response = requests.post(
            f"{api_url}/api/messages/send",
            json={
                "sender_id": sender_id,
                "session_id": session_id,
                "content": content
            },
            timeout=10
        )

        latency = (time.time() - start_time) * 1000  # convert to ms

        if response.status_code == 200:
            stats.add_send(latency)
            data = response.json()
            message = data.get('message', {})
            return message.get('message_id'), latency
        else:
            stats.add_error(f"HTTP {response.status_code}")
            return None, latency

    except Exception as e:
        latency = (time.time() - start_time) * 1000
        stats.add_error(str(e))
        return None, latency

def run_experiment(api_url, ws_url, num_users, msg_per_sec, duration_sec):
    """运行吞吐量测试"""
    print(f"\n{'='*60}")
    print(f"Experiment 2: Throughput Scaling Test")
    print(f"{'='*60}")
    print(f"Users: {num_users}")
    print(f"Target: {msg_per_sec} msg/s")
    print(f"Duration: {duration_sec}s")
    print(f"{'='*60}\n")

    stats = ThroughputStats()

    # 1. 创建一个测试群
    print("[Setup] Creating test group...")
    member_ids = [f"throughput_user_{i}" for i in range(num_users)]
    create_resp = requests.post(
        f"{api_url}/api/sessions/group",
        json={
            "creator_id": member_ids[0],
            "name": f"throughput_test_{int(time.time())}",
            "member_ids": member_ids
        }
    )

    if create_resp.status_code != 200:
        print(f"✗ Failed to create group: {create_resp.text}")
        return None

    session_id = create_resp.json()['session']['session_id']
    print(f"✓ Created session: {session_id}")

    # 2. 启动一些WebSocket接收器（采样，不是所有用户）
    print(f"[Setup] Starting {min(10, num_users)} WebSocket receivers...")
    receivers = []
    for i in range(min(10, num_users)):
        receiver = WSReceiver(ws_url, member_ids[i], stats)
        receiver.start()
        receivers.append(receiver)

    time.sleep(2)

    # 3. 开始发送消息
    print(f"[Test] Sending messages at {msg_per_sec} msg/s for {duration_sec}s...")
    start_time = time.time()
    message_count = 0
    interval = 1.0 / msg_per_sec  # 每条消息间隔

    while time.time() - start_time < duration_sec:
        # 轮流让不同用户发送
        sender_id = member_ids[message_count % num_users]
        content = f"msg_{message_count}_at_{time.time():.3f}"

        msg_id, latency = send_message(api_url, sender_id, session_id, content, stats)

        if msg_id and receivers:
            # 通知接收器记录这条消息
            receivers[0].register_message(msg_id)

        message_count += 1

        # 显示进度
        if message_count % 100 == 0:
            elapsed = time.time() - start_time
            actual_rate = message_count / elapsed
            current_stats = stats.get_stats()
            avg_latency = current_stats.get('send_latency', {}).get('avg', 0)
            print(f"  Progress: {message_count} msgs, "
                  f"actual rate: {actual_rate:.1f} msg/s, "
                  f"avg latency: {avg_latency:.1f}ms, "
                  f"errors: {current_stats['total_errors']}")

        # 控制发送速率
        time.sleep(max(0, interval))

    total_time = time.time() - start_time

    # 4. 等待接收器收到所有消息
    print(f"[Wait] Waiting for messages to be received...")
    time.sleep(5)

    # 5. 停止接收器
    for receiver in receivers:
        receiver.stop()

    # 6. 统计结果
    final_stats = stats.get_stats()
    actual_throughput = final_stats['messages_sent'] / total_time

    print(f"\n{'='*60}")
    print(f"RESULTS")
    print(f"{'='*60}")
    print(f"Target throughput: {msg_per_sec} msg/s")
    print(f"Actual throughput: {actual_throughput:.2f} msg/s")
    print(f"Messages sent: {final_stats['messages_sent']}")
    print(f"Messages received (sampled): {final_stats['messages_received']}")
    print(f"Total errors: {final_stats['total_errors']}")
    print(f"Error rate: {final_stats.get('error_rate', 0):.2f}%")

    if 'send_latency' in final_stats:
        print(f"\nSend Latency:")
        print(f"  Avg: {final_stats['send_latency']['avg']:.2f}ms")
        print(f"  Median: {final_stats['send_latency']['median']:.2f}ms")
        print(f"  P95: {final_stats['send_latency']['p95']:.2f}ms")
        print(f"  P99: {final_stats['send_latency']['p99']:.2f}ms")
        print(f"  Max: {final_stats['send_latency']['max']:.2f}ms")

    if 'e2e_latency' in final_stats:
        print(f"\nEnd-to-End Latency (sampled):")
        print(f"  Avg: {final_stats['e2e_latency']['avg']:.2f}ms")
        print(f"  P95: {final_stats['e2e_latency']['p95']:.2f}ms")
        print(f"  P99: {final_stats['e2e_latency']['p99']:.2f}ms")

    if final_stats['errors']:
        print(f"\nError breakdown:")
        for error, count in final_stats['errors'].items():
            print(f"  - {error}: {count}")

    print(f"{'='*60}\n")

    # 保存结果
    result = {
        'experiment': 'throughput_scaling',
        'timestamp': datetime.now().isoformat(),
        'config': {
            'num_users': num_users,
            'target_msg_per_sec': msg_per_sec,
            'duration_sec': duration_sec
        },
        'results': final_stats,
        'actual_throughput': actual_throughput,
        'session_id': session_id
    }

    filename = f"experiment2_results_{num_users}users_{msg_per_sec}mps_{datetime.now().strftime('%Y%m%d_%H%M%S')}.json"
    with open(filename, 'w') as f:
        json.dump(result, f, indent=2)

    print(f"Results saved to: {filename}\n")

    return result

def main():
    parser = argparse.ArgumentParser(description='Experiment 2: Throughput Scaling Test')
    parser.add_argument('--users', type=int, default=500,
                        help='Number of users (default: 500)')
    parser.add_argument('--msg-per-sec', type=int, default=10,
                        help='Target messages per second (default: 10)')
    parser.add_argument('--duration', type=int, default=60,
                        help='Test duration in seconds (default: 60)')
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
        msg_per_sec=args.msg_per_sec,
        duration_sec=args.duration
    )

if __name__ == '__main__':
    main()
