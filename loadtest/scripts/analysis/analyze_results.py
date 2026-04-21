#!/usr/bin/env python3
"""
负载测试结果分析和可视化工具
自动生成图表和报告，用于PPT展示
"""

import json
import sys
import glob
import matplotlib.pyplot as plt
import pandas as pd
from datetime import datetime
import os

# 设置中文字体（Mac）
plt.rcParams['font.sans-serif'] = ['Arial Unicode MS', 'SimHei', 'DejaVu Sans']
plt.rcParams['axes.unicode_minus'] = False

class ResultAnalyzer:
    def __init__(self, output_dir='analysis_output'):
        self.output_dir = output_dir
        os.makedirs(output_dir, exist_ok=True)
        print(f"\n📁 分析结果将保存到: {output_dir}/\n")

    def analyze_experiment1(self, result_files):
        """分析实验1：连接扩展测试"""
        print("=" * 60)
        print("实验1：连接扩展测试 - 分析结果")
        print("=" * 60)

        data = []
        for file in result_files:
            with open(file, 'r') as f:
                result = json.load(f)
                config = result['config']
                stats = result['results']
                data.append({
                    'connections': config['target_connections'],
                    'connected': stats['connected'],
                    'failed': stats['failed'],
                    'success_rate': stats['success_rate'],
                    'avg_time': stats['avg_connection_time'],
                    'max_time': stats['max_connection_time']
                })

        df = pd.DataFrame(data).sort_values('connections')

        # 打印表格
        print("\n连接数 | 成功 | 失败 | 成功率 | 平均连接时间 | 最大连接时间")
        print("-" * 70)
        for _, row in df.iterrows():
            print(f"{int(row['connections']):6d} | {int(row['connected']):4d} | {int(row['failed']):4d} | "
                  f"{row['success_rate']:6.1f}% | {row['avg_time']:12.3f}s | {row['max_time']:12.3f}s")

        # 生成图表
        fig, axes = plt.subplots(2, 2, figsize=(14, 10))
        fig.suptitle('Experiment 1: Connection Scaling Test', fontsize=16, fontweight='bold')

        # 图1：成功率
        axes[0, 0].plot(df['connections'], df['success_rate'], 'o-', linewidth=2, markersize=8, color='#2ecc71')
        axes[0, 0].set_xlabel('Number of Connections', fontsize=12)
        axes[0, 0].set_ylabel('Success Rate (%)', fontsize=12)
        axes[0, 0].set_title('Connection Success Rate', fontsize=14)
        axes[0, 0].grid(True, alpha=0.3)
        axes[0, 0].set_ylim([0, 105])

        # 图2：成功vs失败
        width = 0.35
        x = range(len(df))
        axes[0, 1].bar([i - width/2 for i in x], df['connected'], width, label='Connected', color='#3498db')
        axes[0, 1].bar([i + width/2 for i in x], df['failed'], width, label='Failed', color='#e74c3c')
        axes[0, 1].set_xlabel('Target Connections', fontsize=12)
        axes[0, 1].set_ylabel('Count', fontsize=12)
        axes[0, 1].set_title('Connected vs Failed', fontsize=14)
        axes[0, 1].set_xticks(x)
        axes[0, 1].set_xticklabels(df['connections'])
        axes[0, 1].legend()
        axes[0, 1].grid(True, alpha=0.3, axis='y')

        # 图3：连接时间
        axes[1, 0].plot(df['connections'], df['avg_time'] * 1000, 'o-', linewidth=2, markersize=8,
                       color='#9b59b6', label='Average')
        axes[1, 0].plot(df['connections'], df['max_time'] * 1000, 's--', linewidth=2, markersize=8,
                       color='#e67e22', label='Maximum')
        axes[1, 0].set_xlabel('Number of Connections', fontsize=12)
        axes[1, 0].set_ylabel('Connection Time (ms)', fontsize=12)
        axes[1, 0].set_title('Connection Latency', fontsize=14)
        axes[1, 0].legend()
        axes[1, 0].grid(True, alpha=0.3)

        # 图4：汇总表格
        axes[1, 1].axis('tight')
        axes[1, 1].axis('off')
        table_data = []
        for _, row in df.iterrows():
            table_data.append([
                f"{row['connections']}",
                f"{row['success_rate']:.1f}%",
                f"{row['avg_time']*1000:.0f}ms"
            ])
        table = axes[1, 1].table(cellText=table_data,
                                colLabels=['Connections', 'Success Rate', 'Avg Time'],
                                cellLoc='center',
                                loc='center')
        table.auto_set_font_size(False)
        table.set_fontsize(10)
        table.scale(1, 2)
        axes[1, 1].set_title('Summary', fontsize=14)

        plt.tight_layout()
        output_file = f"{self.output_dir}/experiment1_connection_scaling.png"
        plt.savefig(output_file, dpi=300, bbox_inches='tight')
        print(f"\n✅ 图表已保存: {output_file}")
        plt.close()

        return df

    def analyze_experiment2(self, result_files):
        """分析实验2：吞吐量测试"""
        print("\n" + "=" * 60)
        print("实验2：吞吐量测试 - 分析结果")
        print("=" * 60)

        data = []
        for file in result_files:
            with open(file, 'r') as f:
                result = json.load(f)
                config = result['config']
                stats = result['results']

                send_latency = stats.get('send_latency', {})
                data.append({
                    'users': config['num_users'],
                    'target_mps': config['target_msg_per_sec'],
                    'actual_mps': result.get('actual_throughput', 0),
                    'msg_sent': stats['messages_sent'],
                    'error_rate': stats.get('error_rate', 0),
                    'avg_latency': send_latency.get('avg', 0),
                    'p95_latency': send_latency.get('p95', 0),
                    'p99_latency': send_latency.get('p99', 0)
                })

        df = pd.DataFrame(data).sort_values('target_mps')

        # 打印表格
        print("\n目标速率 | 实际速率 | 平均延迟 | P95延迟 | P99延迟 | 错误率")
        print("-" * 70)
        for _, row in df.iterrows():
            print(f"{int(row['target_mps']):8d} | {row['actual_mps']:8.1f} | "
                  f"{row['avg_latency']:8.1f}ms | {row['p95_latency']:7.1f}ms | "
                  f"{row['p99_latency']:7.1f}ms | {row['error_rate']:6.2f}%")

        # 生成图表
        fig, axes = plt.subplots(2, 2, figsize=(14, 10))
        fig.suptitle('Experiment 2: Throughput Scaling Test', fontsize=16, fontweight='bold')

        # 图1：吞吐量对比
        x = range(len(df))
        width = 0.35
        axes[0, 0].bar([i - width/2 for i in x], df['target_mps'], width, label='Target', color='#3498db')
        axes[0, 0].bar([i + width/2 for i in x], df['actual_mps'], width, label='Actual', color='#2ecc71')
        axes[0, 0].set_xlabel('Test Scenario', fontsize=12)
        axes[0, 0].set_ylabel('Messages per Second', fontsize=12)
        axes[0, 0].set_title('Target vs Actual Throughput', fontsize=14)
        axes[0, 0].set_xticks(x)
        axes[0, 0].set_xticklabels([f"{int(mps)} msg/s" for mps in df['target_mps']])
        axes[0, 0].legend()
        axes[0, 0].grid(True, alpha=0.3, axis='y')

        # 图2：延迟分布
        axes[0, 1].plot(df['target_mps'], df['avg_latency'], 'o-', linewidth=2, markersize=8,
                       color='#9b59b6', label='Average')
        axes[0, 1].plot(df['target_mps'], df['p95_latency'], 's-', linewidth=2, markersize=8,
                       color='#e67e22', label='P95')
        axes[0, 1].plot(df['target_mps'], df['p99_latency'], '^-', linewidth=2, markersize=8,
                       color='#e74c3c', label='P99')
        axes[0, 1].set_xlabel('Target Throughput (msg/s)', fontsize=12)
        axes[0, 1].set_ylabel('Latency (ms)', fontsize=12)
        axes[0, 1].set_title('Latency Distribution', fontsize=14)
        axes[0, 1].legend()
        axes[0, 1].grid(True, alpha=0.3)

        # 图3：错误率
        axes[1, 0].bar(x, df['error_rate'], color='#e74c3c', alpha=0.7)
        axes[1, 0].set_xlabel('Test Scenario', fontsize=12)
        axes[1, 0].set_ylabel('Error Rate (%)', fontsize=12)
        axes[1, 0].set_title('Error Rate by Throughput', fontsize=14)
        axes[1, 0].set_xticks(x)
        axes[1, 0].set_xticklabels([f"{int(mps)} msg/s" for mps in df['target_mps']])
        axes[1, 0].grid(True, alpha=0.3, axis='y')

        # 图4：汇总表格
        axes[1, 1].axis('tight')
        axes[1, 1].axis('off')
        table_data = []
        for _, row in df.iterrows():
            table_data.append([
                f"{row['target_mps']:.0f}",
                f"{row['actual_mps']:.1f}",
                f"{row['p95_latency']:.0f}ms",
                f"{row['error_rate']:.2f}%"
            ])
        table = axes[1, 1].table(cellText=table_data,
                                colLabels=['Target', 'Actual', 'P95', 'Error%'],
                                cellLoc='center',
                                loc='center')
        table.auto_set_font_size(False)
        table.set_fontsize(10)
        table.scale(1, 2)
        axes[1, 1].set_title('Summary', fontsize=14)

        plt.tight_layout()
        output_file = f"{self.output_dir}/experiment2_throughput.png"
        plt.savefig(output_file, dpi=300, bbox_inches='tight')
        print(f"\n✅ 图表已保存: {output_file}")
        plt.close()

        return df

    def analyze_experiment3(self, result_files):
        """分析实验3：扇出压力测试"""
        print("\n" + "=" * 60)
        print("实验3：扇出压力测试 - 分析结果")
        print("=" * 60)

        data = []
        for file in result_files:
            with open(file, 'r') as f:
                result = json.load(f)
                config = result['config']
                stats = result['results']

                broadcast = stats.get('broadcast_time', {})
                individual = stats.get('individual_latency', {})

                data.append({
                    'group_size': config['group_size'],
                    'messages': config['num_messages'],
                    'broadcast_avg': broadcast.get('avg', 0),
                    'broadcast_p95': broadcast.get('p95', 0),
                    'broadcast_max': broadcast.get('max', 0),
                    'individual_avg': individual.get('avg', 0),
                    'individual_p95': individual.get('p95', 0),
                    'total_received': stats['total_received']
                })

        df = pd.DataFrame(data).sort_values('group_size')

        # 打印表格
        print("\n群大小 | 消息数 | 广播延迟(平均) | 广播延迟(P95) | 广播延迟(最大) | 总接收数")
        print("-" * 80)
        for _, row in df.iterrows():
            print(f"{int(row['group_size']):6d} | {int(row['messages']):6d} | "
                  f"{row['broadcast_avg']:14.1f}ms | {row['broadcast_p95']:13.1f}ms | "
                  f"{row['broadcast_max']:14.1f}ms | {int(row['total_received']):8d}")

        # 生成图表
        fig, axes = plt.subplots(2, 2, figsize=(14, 10))
        fig.suptitle('Experiment 3: Fan-out Stress Test (Group Chat)', fontsize=16, fontweight='bold')

        # 图1：广播延迟 vs 群大小
        axes[0, 0].plot(df['group_size'], df['broadcast_avg'], 'o-', linewidth=2, markersize=8,
                       color='#3498db', label='Average')
        axes[0, 0].plot(df['group_size'], df['broadcast_p95'], 's-', linewidth=2, markersize=8,
                       color='#e67e22', label='P95')
        axes[0, 0].plot(df['group_size'], df['broadcast_max'], '^-', linewidth=2, markersize=8,
                       color='#e74c3c', label='Maximum')
        axes[0, 0].set_xlabel('Group Size (Users)', fontsize=12)
        axes[0, 0].set_ylabel('Broadcast Latency (ms)', fontsize=12)
        axes[0, 0].set_title('Broadcast Latency vs Group Size', fontsize=14)
        axes[0, 0].legend()
        axes[0, 0].grid(True, alpha=0.3)

        # 图2：单个用户延迟
        axes[0, 1].bar(range(len(df)), df['individual_avg'], color='#9b59b6', alpha=0.7, label='Average')
        axes[0, 1].bar(range(len(df)), df['individual_p95'], color='#e67e22', alpha=0.5, label='P95')
        axes[0, 1].set_xlabel('Group Size', fontsize=12)
        axes[0, 1].set_ylabel('Individual User Latency (ms)', fontsize=12)
        axes[0, 1].set_title('Per-User Message Reception Latency', fontsize=14)
        axes[0, 1].set_xticks(range(len(df)))
        axes[0, 1].set_xticklabels(df['group_size'])
        axes[0, 1].legend()
        axes[0, 1].grid(True, alpha=0.3, axis='y')

        # 图3：扇出倍数
        fanout_multiplier = df['total_received'] / df['messages']
        axes[1, 0].bar(range(len(df)), fanout_multiplier, color='#2ecc71', alpha=0.7)
        axes[1, 0].set_xlabel('Group Size', fontsize=12)
        axes[1, 0].set_ylabel('Average Recipients per Message', fontsize=12)
        axes[1, 0].set_title('Fan-out Multiplier', fontsize=14)
        axes[1, 0].set_xticks(range(len(df)))
        axes[1, 0].set_xticklabels(df['group_size'])
        axes[1, 0].grid(True, alpha=0.3, axis='y')

        # 图4：汇总表格
        axes[1, 1].axis('tight')
        axes[1, 1].axis('off')
        table_data = []
        for _, row in df.iterrows():
            table_data.append([
                f"{row['group_size']}",
                f"{row['broadcast_avg']:.0f}ms",
                f"{row['broadcast_p95']:.0f}ms",
                f"{row['broadcast_max']:.0f}ms"
            ])
        table = axes[1, 1].table(cellText=table_data,
                                colLabels=['Size', 'Avg', 'P95', 'Max'],
                                cellLoc='center',
                                loc='center')
        table.auto_set_font_size(False)
        table.set_fontsize(10)
        table.scale(1, 2)
        axes[1, 1].set_title('Broadcast Latency Summary', fontsize=14)

        plt.tight_layout()
        output_file = f"{self.output_dir}/experiment3_fanout.png"
        plt.savefig(output_file, dpi=300, bbox_inches='tight')
        print(f"\n✅ 图表已保存: {output_file}")
        plt.close()

        return df

    def generate_summary_report(self):
        """生成汇总报告"""
        report_file = f"{self.output_dir}/SUMMARY_REPORT.md"

        with open(report_file, 'w') as f:
            f.write("# 负载测试结果汇总报告\n\n")
            f.write(f"生成时间: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}\n\n")
            f.write("---\n\n")

            f.write("## 图表文件\n\n")
            f.write("以下图表已生成，可直接用于PPT：\n\n")

            for img in sorted(glob.glob(f"{self.output_dir}/*.png")):
                img_name = os.path.basename(img)
                f.write(f"- `{img_name}`\n")

            f.write("\n---\n\n")
            f.write("## 使用建议\n\n")
            f.write("1. 所有图表为PNG格式，分辨率300 DPI，适合PPT使用\n")
            f.write("2. 图表包含中英文标签，可根据需要编辑\n")
            f.write("3. 建议在PPT中每个实验使用一张图表\n")
            f.write("4. 原始JSON数据文件可用于进一步分析\n")

        print(f"\n✅ 汇总报告已保存: {report_file}")

def main():
    print("=" * 60)
    print("  负载测试结果分析工具")
    print("=" * 60)

    analyzer = ResultAnalyzer()

    # 查找所有结果文件
    exp1_files = sorted(glob.glob("experiment1_results_*.json"))
    exp2_files = sorted(glob.glob("experiment2_results_*.json"))
    exp3_files = sorted(glob.glob("experiment3_results_*.json"))

    if not any([exp1_files, exp2_files, exp3_files]):
        print("\n❌ 没有找到结果文件！")
        print("请先运行负载测试，生成结果文件。")
        sys.exit(1)

    # 分析各个实验
    if exp1_files:
        print(f"\n找到 {len(exp1_files)} 个实验1结果文件")
        analyzer.analyze_experiment1(exp1_files)

    if exp2_files:
        print(f"\n找到 {len(exp2_files)} 个实验2结果文件")
        analyzer.analyze_experiment2(exp2_files)

    if exp3_files:
        print(f"\n找到 {len(exp3_files)} 个实验3结果文件")
        analyzer.analyze_experiment3(exp3_files)

    # 生成汇总报告
    analyzer.generate_summary_report()

    print("\n" + "=" * 60)
    print("✅ 分析完成！")
    print("=" * 60)
    print(f"\n📊 所有图表和报告已保存到: {analyzer.output_dir}/")
    print("\n可以直接将PNG图片拖入PPT中使用。")
    print("=" * 60)

if __name__ == '__main__':
    main()
