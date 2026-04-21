#!/usr/bin/env python3
"""
Experiment 3: Fan-out Stress Test (Group Chat)
测试大群聊的消息广播性能
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

class FanoutStats:
    def __init__(self):
        self.lock = threading.Lock()
        self.messages_sent = 0
        self.total_received = 0
        self.broadcast_times = []  # 每条消息从发送到最后一个人收到的时间
        self.individual_latencies = []  # 每个用户收到消息的延迟
        self.errors = defaultdict(int)
        self.reception_data = {}  # msg_id -> {send_time, receivers: {user_id: recv_time}}

    def register_send(self, msg_id, send_time):
        with self.lock:
            self.messages_sent += 1
            self.reception_data[msg_id] = {
                'send_time': send_time,
                'receivers': {}
            }

    def register_receive(self, msg_id, user_id, recv_time):
        with self.lock:
            if msg_id in self.reception_data:
                self.reception_data[msg_id]['receivers'][user_id] = recv_time
                self.total_received += 1

                # 计算单个用户的接收延迟
                send_time = self.reception_data[msg_id]['send_time']
                latency = (recv_time - send_time) * 1000  # ms
                self.individual_latencies.append(latency)

    def add_error(self, error_msg):
        with self.lock:
            self.errors[error_msg] += 1

    def calculate_broadcast_times(self):
        """计算每条消息的完整广播时间"""
        with self.lock:
            for msg_id, data in self.reception_data.items():
                receivers = data['receivers']
                if receivers:
                    send_time = data['send_time']
                    last_recv_time = max(receivers.values())
                    broadcast_time = (last_recv_time - send_time) * 1000  # ms
                    self.broadcast_times.append(broadcast_time)

    def get_stats(self):
        self.calculate_broadcast_times()

        with self.lock:
            stats = {
                'messages_sent': self.messages_sent,
                'total_received': self.total_received,
                'errors': dict(self.errors),
                'total_errors': sum(self.errors.values())
            }

            # 广播时间统计（最重要的指标）
            if self.broadcast_times:
                stats['broadcast_time'] = {
                    'avg': statistics.mean(self.broadcast_times),
                    'median': statistics.median(self.broadcast_times),
                    'p95': self._percentile(self.broadcast_times, 95),
                    'p99': self._percentile(self.broadcast_times, 99),
                    'max': max(self.broadcast_times),
                    'min': min(self.broadcast_times)
                }

            # 单个用户延迟统计
            if self.individual_latencies:
                stats['individual_latency'] = {
                    'avg': statistics.mean(self.individual_latencies),
                    'median': statistics.median(self.individual_latencies),
                    'p95': self._percentile(self.individual_latencies, 95),
                    'p99': self._percentile(self.individual_latencies, 99),
                    'max': max(self.individual_latencies),
                    'min': min(self.individual_latencies)
                }

            # 接收率统计
            if self.messages_sent > 0:
                # 为每条消息计算接收率
                reception_rates = []
                for msg_id, data in self.reception_data.items():
                    num_receivers = len(data['receivers'])
                    reception_rates.append(num_receivers)

                if reception_rates:
                    stats['reception_rate'] = {
                        'avg_receivers_per_msg': statistics.mean(reception_rates),
                        'min_receivers': min(reception_rates),
                        'max_receivers': max(reception_rates)
                    }

            return stats

    def _percentile(self, data, percentile):
        sorted_data = sorted(data)
        index = int(len(sorted_data) * percentile / 100)
        return sorted_data[min(index, len(sorted_data) - 1)]

class FanoutReceiver:
    """群聊接收器"""
    def __init__(self, ws_url, user_id, stats):
        self.ws_url = ws_url
        self.user_id = user_id
        self.stats = stats
        self.ws = None
        self.running = False

    def start(self):
        self.running = True
        thread = threading.Thread(target=self._run)
        thread.daemon = True
        thread.start()
        time.sleep(0.1)

    def _run(self):
        try:
            self.ws = websocket.create_connection(self.ws_url, timeout=10)

            # 认证
            auth_msg = json.dumps({"type": "auth", "user_id": self.user_id})
            self.ws.send(auth_msg)

            response = self.ws.recv()
            resp_data = json.loads(response)

            if resp_data.get('type') != 'auth_ok':
                print(f"✗ {self.user_id} auth failed")
                return

            # 接收消息循环
            while self.running:
                try:
                    self.ws.settimeout(1.0)
                    message = self.ws.recv()
                    recv_time = time.time()

                    data = json.loads(message)
                    if data.get('type') == 'new_message':
                        msg = data.get('message', {})
                        msg_id = msg.get('message_id')
                        self.stats.register_receive(msg_id, self.user_id, recv_time)

                except websocket.WebSocketTimeoutException:
                    continue
                except Exception as e:
                    if self.running:
                        self.stats.add_error(f"Receive error: {str(e)}")
                    break

        except Exception as e:
            self.stats.add_error(f"Connection error: {str(e)}")
        finally:
            if self.ws:
                self.ws.close()

    def stop(self):
        self.running = False

def run_experiment(api_url, ws_url, group_size, num_messages):
    """运行扇出压力测试"""
    print(f"\n{'='*60}")
    print(f"Experiment 3: Fan-out Stress Test")
    print(f"{'='*60}")
    print(f"Group size: {group_size}")
    print(f"Messages: {num_messages}")
    print(f"{'='*60}\n")

    stats = FanoutStats()

    # 1. 创建大群
    print(f"[Setup] Creating group with {group_size} members...")
    member_ids = [f"fanout_user_{i}" for i in range(group_size)]

    create_resp = requests.post(
        f"{api_url}/api/sessions/group",
        json={
            "creator_id": member_ids[0],
            "name": f"fanout_test_{group_size}_{int(time.time())}",
            "member_ids": member_ids
        },
        timeout=30
    )

    if create_resp.status_code != 200:
        print(f"✗ Failed to create group: {create_resp.text}")
        return None

    session_id = create_resp.json()['session']['session_id']
    print(f"✓ Created session: {session_id}")

    # 2. 启动所有WebSocket接收器
    print(f"[Setup] Starting {group_size} WebSocket receivers...")
    receivers = []
    batch_size = 20  # 分批启动，避免瞬间压力太大

    for i in range(0, group_size, batch_size):
        batch_end = min(i + batch_size, group_size)
        print(f"  Starting receivers {i} to {batch_end}...")

        for j in range(i, batch_end):
            receiver = FanoutReceiver(ws_url, member_ids[j], stats)
            receiver.start()
            receivers.append(receiver)
            time.sleep(0.01)  # 稍微错开

        time.sleep(1)

    print(f"✓ All receivers started")
    time.sleep(2)  # 确保所有连接建立

    # 3. 发送消息
    print(f"[Test] Sending {num_messages} messages...")
    sender_id = member_ids[0]

    for i in range(num_messages):
        send_time = time.time()
        content = f"fanout_test_msg_{i}_at_{send_time:.3f}"

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

            if response.status_code == 200:
                data = response.json()
                msg_id = data.get('message', {}).get('message_id')
                stats.register_send(msg_id, send_time)
                print(f"  ✓ Message {i+1}/{num_messages} sent (id: {msg_id})")
            else:
                stats.add_error(f"HTTP {response.status_code}")
                print(f"  ✗ Message {i+1} failed: HTTP {response.status_code}")

        except Exception as e:
            stats.add_error(str(e))
            print(f"  ✗ Message {i+1} error: {e}")

        # 消息之间间隔，避免太快
        time.sleep(2)

    # 4. 等待消息被接收
    print(f"[Wait] Waiting for messages to be received...")
    time.sleep(10)

    # 5. 停止所有接收器
    print(f"[Cleanup] Stopping receivers...")
    for receiver in receivers:
        receiver.stop()

    time.sleep(1)

    # 6. 统计结果
    final_stats = stats.get_stats()

    print(f"\n{'='*60}")
    print(f"RESULTS")
    print(f"{'='*60}")
    print(f"Group size: {group_size}")
    print(f"Messages sent: {final_stats['messages_sent']}")
    print(f"Total receptions: {final_stats['total_received']}")
    print(f"Expected receptions: {final_stats['messages_sent'] * group_size}")
    print(f"Reception rate: {final_stats['total_received'] / (final_stats['messages_sent'] * group_size) * 100:.2f}%" if final_stats['messages_sent'] > 0 else "N/A")

    if 'broadcast_time' in final_stats:
        print(f"\nBroadcast Time (send → last user received):")
        print(f"  Avg: {final_stats['broadcast_time']['avg']:.2f}ms")
        print(f"  Median: {final_stats['broadcast_time']['median']:.2f}ms")
        print(f"  P95: {final_stats['broadcast_time']['p95']:.2f}ms")
        print(f"  P99: {final_stats['broadcast_time']['p99']:.2f}ms")
        print(f"  Max: {final_stats['broadcast_time']['max']:.2f}ms")

    if 'individual_latency' in final_stats:
        print(f"\nIndividual User Latency:")
        print(f"  Avg: {final_stats['individual_latency']['avg']:.2f}ms")
        print(f"  P95: {final_stats['individual_latency']['p95']:.2f}ms")
        print(f"  P99: {final_stats['individual_latency']['p99']:.2f}ms")
        print(f"  Max: {final_stats['individual_latency']['max']:.2f}ms")

    if 'reception_rate' in final_stats:
        print(f"\nReception Rate:")
        print(f"  Avg receivers per message: {final_stats['reception_rate']['avg_receivers_per_msg']:.1f}/{group_size}")
        print(f"  Min receivers: {final_stats['reception_rate']['min_receivers']}")
        print(f"  Max receivers: {final_stats['reception_rate']['max_receivers']}")

    if final_stats['errors']:
        print(f"\nErrors:")
        for error, count in final_stats['errors'].items():
            print(f"  - {error}: {count}")

    print(f"{'='*60}\n")

    # 保存结果
    result = {
        'experiment': 'fanout_stress_test',
        'timestamp': datetime.now().isoformat(),
        'config': {
            'group_size': group_size,
            'num_messages': num_messages
        },
        'results': final_stats,
        'session_id': session_id
    }

    filename = f"experiment3_results_{group_size}users_{num_messages}msgs_{datetime.now().strftime('%Y%m%d_%H%M%S')}.json"
    with open(filename, 'w') as f:
        json.dump(result, f, indent=2)

    print(f"Results saved to: {filename}\n")

    return result

def main():
    parser = argparse.ArgumentParser(description='Experiment 3: Fan-out Stress Test')
    parser.add_argument('--group-size', type=int, default=50,
                        help='Number of users in the group (default: 50)')
    parser.add_argument('--messages', type=int, default=10,
                        help='Number of messages to send (default: 10)')
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
        group_size=args.group_size,
        num_messages=args.messages
    )

if __name__ == '__main__':
    main()
