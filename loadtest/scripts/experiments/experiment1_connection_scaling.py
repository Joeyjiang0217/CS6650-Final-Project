#!/usr/bin/env python3
"""
Experiment 1: Connection Scaling Test
测试系统能支持多少并发WebSocket连接
"""

import argparse
import json
import time
import threading
import websocket
from datetime import datetime
from collections import defaultdict
import sys

class ConnectionStats:
    def __init__(self):
        self.lock = threading.Lock()
        self.connected = 0
        self.failed = 0
        self.connection_times = []
        self.errors = defaultdict(int)

    def add_success(self, duration):
        with self.lock:
            self.connected += 1
            self.connection_times.append(duration)

    def add_failure(self, error_msg):
        with self.lock:
            self.failed += 1
            self.errors[error_msg] += 1

    def get_stats(self):
        with self.lock:
            if self.connection_times:
                avg_time = sum(self.connection_times) / len(self.connection_times)
                max_time = max(self.connection_times)
                min_time = min(self.connection_times)
            else:
                avg_time = max_time = min_time = 0

            return {
                'connected': self.connected,
                'failed': self.failed,
                'success_rate': self.connected / (self.connected + self.failed) * 100 if (self.connected + self.failed) > 0 else 0,
                'avg_connection_time': avg_time,
                'max_connection_time': max_time,
                'min_connection_time': min_time,
                'errors': dict(self.errors)
            }

def connect_websocket(ws_url, user_id, stats, keep_alive_seconds):
    """建立WebSocket连接并保持"""
    start_time = time.time()

    try:
        ws = websocket.create_connection(ws_url, timeout=10)

        # 发送认证消息
        auth_msg = json.dumps({
            "type": "auth",
            "user_id": user_id
        })
        ws.send(auth_msg)

        # 等待认证响应
        response = ws.recv()
        resp_data = json.loads(response)

        if resp_data.get('type') == 'auth_ok':
            connection_time = time.time() - start_time
            stats.add_success(connection_time)
            print(f"✓ {user_id} connected in {connection_time:.3f}s")

            # 保持连接
            time.sleep(keep_alive_seconds)

            ws.close()
        else:
            stats.add_failure(f"Auth failed: {response}")
            print(f"✗ {user_id} auth failed")

    except Exception as e:
        stats.add_failure(str(e))
        print(f"✗ {user_id} connection failed: {e}")

def run_experiment(ws_url, num_connections, batch_size=50, keep_alive=60):
    """运行连接扩展测试"""
    print(f"\n{'='*60}")
    print(f"Experiment 1: Connection Scaling Test")
    print(f"{'='*60}")
    print(f"Target connections: {num_connections}")
    print(f"Batch size: {batch_size}")
    print(f"Keep alive: {keep_alive}s")
    print(f"{'='*60}\n")

    stats = ConnectionStats()
    threads = []

    start_time = time.time()

    # 分批建立连接
    for batch_start in range(0, num_connections, batch_size):
        batch_end = min(batch_start + batch_size, num_connections)
        batch_threads = []

        print(f"\n[Batch {batch_start//batch_size + 1}] Connecting {batch_start} - {batch_end}...")

        for i in range(batch_start, batch_end):
            user_id = f"load_test_user_{i}"
            thread = threading.Thread(
                target=connect_websocket,
                args=(ws_url, user_id, stats, keep_alive)
            )
            thread.start()
            batch_threads.append(thread)
            threads.append(thread)

            # 稍微错开连接时间
            time.sleep(0.01)

        # 等待当前批次完成连接（不等待keep_alive）
        time.sleep(2)

        # 显示当前进度
        current_stats = stats.get_stats()
        print(f"Progress: {current_stats['connected']}/{num_connections} connected, "
              f"{current_stats['failed']} failed, "
              f"success rate: {current_stats['success_rate']:.1f}%")

    # 等待所有线程完成
    print(f"\n[Wait] Keeping connections alive for {keep_alive}s...")
    for thread in threads:
        thread.join()

    total_time = time.time() - start_time

    # 最终统计
    final_stats = stats.get_stats()

    print(f"\n{'='*60}")
    print(f"RESULTS")
    print(f"{'='*60}")
    print(f"Total connections attempted: {num_connections}")
    print(f"Successfully connected: {final_stats['connected']}")
    print(f"Failed: {final_stats['failed']}")
    print(f"Success rate: {final_stats['success_rate']:.2f}%")
    print(f"Avg connection time: {final_stats['avg_connection_time']:.3f}s")
    print(f"Max connection time: {final_stats['max_connection_time']:.3f}s")
    print(f"Min connection time: {final_stats['min_connection_time']:.3f}s")
    print(f"Total test time: {total_time:.1f}s")

    if final_stats['errors']:
        print(f"\nError breakdown:")
        for error, count in final_stats['errors'].items():
            print(f"  - {error}: {count}")

    print(f"{'='*60}\n")

    # 保存结果
    result = {
        'experiment': 'connection_scaling',
        'timestamp': datetime.now().isoformat(),
        'config': {
            'target_connections': num_connections,
            'batch_size': batch_size,
            'keep_alive_seconds': keep_alive
        },
        'results': final_stats,
        'total_time_seconds': total_time
    }

    filename = f"experiment1_results_{num_connections}conn_{datetime.now().strftime('%Y%m%d_%H%M%S')}.json"
    with open(filename, 'w') as f:
        json.dump(result, f, indent=2)

    print(f"Results saved to: {filename}\n")

    return result

def main():
    parser = argparse.ArgumentParser(description='Experiment 1: Connection Scaling Test')
    parser.add_argument('--connections', type=int, default=100,
                        help='Number of concurrent connections (default: 100)')
    parser.add_argument('--batch-size', type=int, default=50,
                        help='Connections per batch (default: 50)')
    parser.add_argument('--keep-alive', type=int, default=60,
                        help='Seconds to keep connections alive (default: 60)')
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

    ws_url = config['ws_url']

    # 运行测试
    run_experiment(
        ws_url=ws_url,
        num_connections=args.connections,
        batch_size=args.batch_size,
        keep_alive=args.keep_alive
    )

if __name__ == '__main__':
    main()
