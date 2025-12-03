#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
歌词处理工具
自动将包含日文和中文翻译的歌词文件分割成独立的行
"""

import re
import os
import argparse
from typing import List, Tuple, Optional

class LyricProcessor:
    def __init__(self):
        self.time_pattern = re.compile(r'^\[(\d{2}:\d{2}\.\d{2})\]')
        self.meta_pattern = re.compile(r'^\[(ti|ar|al):')
    
    def is_meta_line(self, line: str) -> bool:
        """检查是否为元数据行（如标题、艺术家等）"""
        return bool(self.meta_pattern.match(line.strip()))
    
    def extract_timestamp(self, line: str) -> Optional[str]:
        """提取时间戳"""
        match = self.time_pattern.match(line.strip())
        return match.group(1) if match else None
    
    def split_lyric_line(self, line: str) -> Tuple[str, str, str]:
        """
        分割歌词行
        返回: (时间戳, 原文部分, 翻译部分)
        """
        line = line.strip()
        timestamp = self.extract_timestamp(line)
        
        if not timestamp:
            return "", line, ""
        
        # 移除时间戳
        content = line.replace(f'[{timestamp}]', '').strip()
        
        # 检查是否只有时间戳而没有内容（音乐间隙）
        if not content:
            # 如果时间戳后没有内容，直接保留这一行
            return timestamp, "", ""
        
        # 使用分组方法分割：原文（日文/英文）+ 翻译（中文）
        original, translation = self._split_by_language_groups(content)
        
        if original and translation:
            return timestamp, original, translation
        
        # 如果分组方法失败，尝试其他策略
        strategies = [
            self._split_by_space,
            self._split_by_chinese_detection,
            self._split_by_pattern_matching,
            self._split_by_character_analysis
        ]
        
        for strategy in strategies:
            original, translation = strategy(content)
            if original and translation:
                return timestamp, original, translation
        
        # 如果所有方法都失败，返回整行作为原文
        return timestamp, content, ""
    
    def _split_by_language_groups(self, content: str) -> Tuple[str, str]:
        """
        通过语言分组分割：原文（日文/英文）+ 翻译（中文）
        基于用户建议：以空格分段，检查各段是否含有日文/英文（原文）或全是中文（译文）
        改进：处理括号和特殊符号的情况
        """
        # 预处理：处理特殊符号情况
        content = self._preprocess_content(content)
        
        # 按空格分割成词段
        segments = content.split(' ')
        if len(segments) < 2:
            return "", ""
        
        # 寻找最佳分割点
        best_split = self._find_optimal_split_point(segments)
        
        if best_split > 0:
            original_parts = segments[:best_split]
            translation_parts = segments[best_split:]
            
            original_text = ' '.join(original_parts).strip()
            translation_text = ' '.join(translation_parts).strip()
            
            # 验证分割结果
            if (self.contains_original_language(original_text) and
                self.is_pure_chinese(translation_text)):
                return original_text, translation_text
        
        # 如果标准方法失败，尝试特殊符号分割
        return self._try_special_symbol_split(content)
    
    def _preprocess_content(self, content: str) -> str:
        """
        预处理内容，处理特殊符号
        """
        # 处理括号内容的情况
        import re
        
        # 匹配括号内容：(日文) (中文)
        bracket_pattern = r'\([^)]*\)'
        matches = re.findall(bracket_pattern, content)
        
        if matches:
            # 如果找到括号，尝试在括号前后分割
            for match in matches:
                # 检查括号内是否主要是日文或中文
                bracket_content = match[1:-1]  # 去掉括号
                if self.contains_original_language(bracket_content):
                    # 括号内是原文，可能在括号前分割
                    bracket_pos = content.find(match)
                    if bracket_pos > 0:
                        before = content[:bracket_pos].strip()
                        after = content[bracket_pos + len(match):].strip()
                        if before and after:
                            return before + " " + match  # 保留括号
                elif self.is_pure_chinese(bracket_content):
                    # 括号内是中文，可能在括号后分割
                    bracket_pos = content.find(match)
                    if bracket_pos > 0:
                        before = content[:bracket_pos].strip()
                        after = content[bracket_pos + len(match):].strip()
                        if before and after:
                            return before  # 去掉括号，只保留前面部分
        
        return content
    
    def _try_special_symbol_split(self, content: str) -> Tuple[str, str]:
        """
        尝试使用特殊符号进行分割
        专门处理括号情况：(日文) (中文)
        """
        import re
        
        # 首先检查是否包含括号
        if '(' not in content:
            return "", ""
        
        # 查找所有括号对
        bracket_pairs = []
        stack = []
        start = 0
        
        for i, char in enumerate(content):
            if char == '(':
                stack.append(i)
            elif char == ')' and stack:
                start_pos = stack.pop()
                bracket_pairs.append((start_pos, i))
        
        if not bracket_pairs:
            return "", ""
        
        # 尝试每个括号位置作为分割点
        for start_pos, end_pos in bracket_pairs:
            # 获取括号内容
            bracket_content = content[start_pos+1:end_pos]
            
            # 检查括号前的内容
            before_bracket = content[:start_pos].strip()
            # 检查括号后的内容
            after_bracket = content[end_pos+1:].strip()
            
            # 情况1：(日文) 中文
            if self.contains_original_language(bracket_content):
                if after_bracket and self.is_pure_chinese(after_bracket):
                    # 合并括号前的内容和括号内容作为原文
                    original = (before_bracket + ' ' if before_bracket else '') + '(' + bracket_content + ')'
                    translation = after_bracket
                    return original.strip(), translation.strip()
            
            # 情况2：日文 (中文)
            elif self.is_pure_chinese(bracket_content):
                if before_bracket and self.contains_original_language(before_bracket):
                    original = before_bracket
                    translation = '(' + bracket_content + ')' + (' ' + after_bracket if after_bracket else '')
                    return original.strip(), translation.strip()
        
        # 如果括号方法失败，尝试更简单的模式
        simple_patterns = [
            (r'([^(]*)\s*\(([^)]*中文[^)]*)\s*([^)]*)', '日文 (中文) 其他'),
            (r'([^(]*)\s*\(([^)]*)\s*([^)]*中文[^)]*)', '日文 (日文) 中文'),
        ]
        
        for pattern, desc in simple_patterns:
            match = re.search(pattern, content)
            if match:
                groups = match.groups()
                
                if desc == '日文 (中文) 其他':
                    part1 = groups[0].strip()
                    part2 = groups[1].strip()
                    part3 = groups[2].strip()
                    
                    if part1 and part2:
                        original = part1 + ' (' + part2 + ')'
                        translation = part3
                        if self.contains_original_language(original) and self.is_pure_chinese(translation):
                            return original.strip(), translation.strip()
                
                elif desc == '日文 (日文) 中文':
                    part1 = groups[0].strip()
                    part2 = groups[1].strip()
                    part3 = groups[2].strip()
                    
                    if part1 and part3:
                        original = part1 + ' (' + part2 + ')'
                        translation = part3
                        if self.contains_original_language(original) and self.is_pure_chinese(translation):
                            return original.strip(), translation.strip()
        
        return "", ""
    
    def _find_optimal_split_point(self, segments: list) -> int:
        """
        寻找最佳分割点
        策略：找到第一个分割点，使得前面部分含有日文/英文，后面部分全是中文
        """
        for split_point in range(1, len(segments)):
            original_parts = segments[:split_point]
            translation_parts = segments[split_point:]
            
            original_text = ' '.join(original_parts)
            translation_text = ' '.join(translation_parts)
            
            # 检查前面部分是否含有原文语言（日文/英文）
            # 检查后面部分是否全是中文
            if (self.contains_original_language(original_text) and
                self.is_pure_chinese(translation_text)):
                return split_point
        
        # 如果找不到完美的分割点，尝试找最接近的
        best_score = 0
        best_split = 0
        
        for split_point in range(1, len(segments)):
            original_parts = segments[:split_point]
            translation_parts = segments[split_point:]
            
            original_text = ' '.join(original_parts)
            translation_text = ' '.join(translation_parts)
            
            # 计算分数
            original_score = self._calculate_original_language_score(original_text)
            chinese_score = self._calculate_chinese_purity_score(translation_text)
            total_score = original_score + chinese_score
            
            if total_score > best_score:
                best_score = total_score
                best_split = split_point
        
        return best_split if best_score > 0.5 else 0
    
    def contains_original_language(self, text: str) -> bool:
        """
        检查文本是否含有原文语言（日文或英文）
        """
        if not text:
            return False
        
        # 检查是否含有日文字符
        has_japanese = any(self.is_japanese_char(c) for c in text)
        # 检查是否含有英文字符
        has_english = any(self.is_english_char(c) for c in text)
        
        return has_japanese or has_english
    
    def is_pure_chinese(self, text: str) -> bool:
        """
        检查文本是否全是中文（允许少量标点符号）
        """
        if not text:
            return False
        
        chinese_chars = 0
        total_chars = 0
        
        for char in text:
            if char.strip():  # 忽略空格
                total_chars += 1
                if self.is_chinese_char(char):
                    chinese_chars += 1
        
        # 如果90%以上是中文字符，认为是纯中文
        return total_chars > 0 and (chinese_chars / total_chars) > 0.9
    
    def _calculate_original_language_score(self, text: str) -> float:
        """
        计算原文语言分数
        """
        if not text:
            return 0
        
        original_chars = 0
        total_chars = 0
        
        for char in text:
            if char.strip():
                total_chars += 1
                if (self.is_japanese_char(char) or
                    self.is_english_char(char)):
                    original_chars += 1
        
        return (original_chars / total_chars) if total_chars > 0 else 0
    
    def _calculate_chinese_purity_score(self, text: str) -> float:
        """
        计算中文纯度分数
        """
        if not text:
            return 0
        
        chinese_chars = 0
        total_chars = 0
        
        for char in text:
            if char.strip():
                total_chars += 1
                if self.is_chinese_char(char):
                    chinese_chars += 1
        
        return (chinese_chars / total_chars) if total_chars > 0 else 0
    
    def _find_chinese_start_point(self, content: str) -> int:
        """找到中文翻译的起始位置"""
        for i, char in enumerate(content):
            if self.is_chinese_char(char):
                # 检查前面是否主要是原文
                if i > 0:
                    before = content[:i]
                    if self.is_mainly_original(before):
                        return i
        return -1
    
    def _strategy_first_chinese_word(self, parts: list) -> Tuple[str, str]:
        """策略：找到第一个包含中文的词"""
        for i, part in enumerate(parts):
            if self.contains_chinese(part) and i > 0:
                before_parts = parts[:i]
                after_parts = parts[i:]
                
                before_text = ' '.join(before_parts)
                after_text = ' '.join(after_parts)
                
                if (self.is_mainly_original(before_text) and
                    self.is_mainly_translation(after_text)):
                    return before_text.strip(), after_text.strip()
        return "", ""
    
    def _strategy_language_ratio(self, parts: list) -> Tuple[str, str]:
        """策略：基于语言比例的最优分割"""
        best_split = 0
        best_score = 0
        
        for split_point in range(1, len(parts)):
            before_parts = parts[:split_point]
            after_parts = parts[split_point:]
            
            before_text = ' '.join(before_parts)
            after_text = ' '.join(after_parts)
            
            # 计算分割质量分数
            original_score = self._calculate_original_score(before_text)
            translation_score = self._calculate_translation_score(after_text)
            total_score = original_score + translation_score
            
            if total_score > best_score:
                best_score = total_score
                best_split = split_point
        
        if best_score > 1.5:  # 设置阈值
            before_parts = parts[:best_split]
            after_parts = parts[best_split:]
            
            before_text = ' '.join(before_parts)
            after_text = ' '.join(after_parts)
            
            return before_text.strip(), after_text.strip()
        
        return "", ""
    
    def _strategy_progressive_split(self, parts: list) -> Tuple[str, str]:
        """策略：渐进式分割，从最可能的分割点开始"""
        # 优先在较短的词后分割
        for split_point in range(1, min(4, len(parts))):
            before_parts = parts[:split_point]
            after_parts = parts[split_point:]
            
            before_text = ' '.join(before_parts)
            after_text = ' '.join(after_parts)
            
            if (self.is_mainly_original(before_text) and
                self.is_mainly_translation(after_text)):
                return before_text.strip(), after_text.strip()
        
        # 如果短词分割不行，尝试所有可能
        for split_point in range(1, len(parts)):
            before_parts = parts[:split_point]
            after_parts = parts[split_point:]
            
            before_text = ' '.join(before_parts)
            after_text = ' '.join(after_parts)
            
            if (self.is_mainly_original(before_text) and
                self.is_mainly_translation(after_text)):
                return before_text.strip(), after_text.strip()
        
        return "", ""
    
    def _calculate_original_score(self, text: str) -> float:
        """计算原文分数"""
        if not text:
            return 0
        
        original_chars = 0
        total_chars = 0
        
        for char in text:
            if char.strip():
                total_chars += 1
                if (self.is_japanese_char(char) or
                    self.is_english_char(char) or
                    char in 'ー〜。、、'):
                    original_chars += 1
        
        return (original_chars / total_chars) if total_chars > 0 else 0
    
    def _calculate_translation_score(self, text: str) -> float:
        """计算翻译分数"""
        if not text:
            return 0
        
        chinese_chars = 0
        total_chars = 0
        
        for char in text:
            if char.strip():
                total_chars += 1
                if self.is_chinese_char(char):
                    chinese_chars += 1
        
        return (chinese_chars / total_chars) if total_chars > 0 else 0
    
    def is_mainly_original(self, text: str) -> bool:
        """
        检查文本是否主要是原文（日文或英文）
        """
        if not text:
            return False
        
        original_chars = 0
        total_chars = 0
        
        for char in text:
            if char.strip():  # 忽略空格
                total_chars += 1
                if (self.is_japanese_char(char) or
                    self.is_english_char(char) or
                    char in 'ー〜。、、'):
                    original_chars += 1
        
        # 如果超过50%的字符是原文，则认为是原文
        return total_chars > 0 and (original_chars / total_chars) > 0.5
    
    def is_mainly_translation(self, text: str) -> bool:
        """
        检查文本是否主要是翻译（中文）
        """
        if not text:
            return False
        
        chinese_chars = 0
        total_chars = 0
        
        for char in text:
            if char.strip():  # 忽略空格
                total_chars += 1
                if self.is_chinese_char(char):
                    chinese_chars += 1
        
        # 如果超过60%的字符是中文，则认为是翻译
        return total_chars > 0 and (chinese_chars / total_chars) > 0.6
    
    def is_english_char(self, char: str) -> bool:
        """检查字符是否是英文字符"""
        return ('a' <= char <= 'z') or ('A' <= char <= 'Z')
    
    def _split_by_space(self, content: str) -> Tuple[str, str]:
        """通过空格分割"""
        # 单空格分隔
        space_split = content.split(' ', 1)
        if len(space_split) == 2 and self.contains_chinese(space_split[1]):
            # 检查第一部分是否主要是日文
            if self.is_mainly_japanese(space_split[0]):
                return space_split[0].strip(), space_split[1].strip()
        return "", ""
    
    def _split_by_chinese_detection(self, content: str) -> Tuple[str, str]:
        """通过中文检测分割"""
        # 尝试找到第一个中文字符
        for i, char in enumerate(content):
            if self.is_chinese_char(char) and not self.is_japanese_char(char):
                # 检查前面的部分是否主要是日文
                if i > 0 and self.is_mainly_japanese(content[:i]):
                    return content[:i].strip(), content[i:].strip()
        return "", ""
    
    def _split_by_pattern_matching(self, content: str) -> Tuple[str, str]:
        """通过模式匹配分割"""
        # 查找常见的分割模式
        patterns = [
            r'([^\u4e00-\u9fff]+) ([\u4e00-\u9fff].+)',  # 非中文 + 空格 + 中文
            r'([ひらがなカタカナ]+) ([\u4e00-\u9fff].+)',  # 假名 + 空格 + 中文
        ]
        
        import re
        for pattern in patterns:
            match = re.match(pattern, content)
            if match:
                japanese, chinese = match.group(1), match.group(2)
                if self.is_mainly_japanese(japanese) and self.contains_chinese(chinese):
                    return japanese.strip(), chinese.strip()
        
        return "", ""
    
    def _split_by_character_analysis(self, content: str) -> Tuple[str, str]:
        """通过字符分析分割"""
        # 从后往前找，寻找可能的分割点
        for i in range(len(content) - 1, 0, -1):
            if content[i] == ' ' and i < len(content) - 1:
                before = content[:i]
                after = content[i+1:]
                
                # 如果后面部分主要是中文，前面部分主要是日文，则分割
                if self.contains_chinese(after) and self.is_mainly_japanese(before):
                    return before.strip(), after.strip()
        
        return "", ""
    
    def contains_chinese(self, text: str) -> bool:
        """检查文本是否包含中文字符"""
        for char in text:
            if self.is_chinese_char(char):
                return True
        return False
    
    def is_chinese_char(self, char: str) -> bool:
        """检查字符是否是中文字符（排除日文假名）"""
        # 基本中文字符范围
        if '\u4e00' <= char <= '\u9fff':
            # 排除日文假名
            if not ('\u3040' <= char <= '\u309F' or '\u30A0' <= char <= '\u30FF'):
                return True
        return False
    
    def is_mainly_japanese(self, text: str) -> bool:
        """检查文本是否主要是日文"""
        if not text:
            return False
        
        japanese_chars = 0
        total_chars = 0
        
        for char in text:
            if char.strip():  # 忽略空格
                total_chars += 1
                if (self.is_japanese_char(char) or char in 'ー〜。、'):
                    japanese_chars += 1
        
        # 如果超过60%的字符是日文，则认为是日文
        return total_chars > 0 and (japanese_chars / total_chars) > 0.6
    
    def is_japanese_char(self, char: str) -> bool:
        """检查字符是否是日文字符"""
        # 平假名
        if '\u3040' <= char <= '\u309F':
            return True
        # 片假名
        if '\u30A0' <= char <= '\u30FF':
            return True
        # 日文汉字（这里简化处理，实际日文汉字和中文汉字有重叠）
        # 我们通过上下文来判断，而不是单个字符
        return False
    
    def process_file(self, input_file: str, output_file: str = None) -> str:
        """
        处理歌词文件
        """
        if output_file is None:
            base_name = os.path.splitext(input_file)[0]
            file_ext = os.path.splitext(input_file)[1]  # 保留原始扩展名
            output_file = f"{base_name}_split{file_ext}"
        
        try:
            with open(input_file, 'r', encoding='utf-8') as f:
                lines = f.readlines()
        except FileNotFoundError:
            raise FileNotFoundError(f"找不到输入文件: {input_file}")
        except UnicodeDecodeError:
            raise ValueError(f"无法解码文件 {input_file}，请确保文件使用UTF-8编码")
        
        processed_lines = []
        
        for line in lines:
            original_line = line.rstrip('\n\r')  # 保留原始行的格式
            line = line.strip()
            
            # 如果是空行，直接保留（保持原始格式）
            if not line:
                processed_lines.append(original_line)
                continue
            
            # 预处理：删除版权声明
            line = self._remove_copyright_notices(line)
            
            # 如果是元数据行，直接添加
            if self.is_meta_line(line):
                processed_lines.append(line)
                continue
            
            # 尝试分割歌词行
            timestamp, japanese, chinese = self.split_lyric_line(line)
            
            if not timestamp:
                # 没有时间戳的行，直接添加
                processed_lines.append(line)
            else:
                # 有时间戳的行，分割日文和中文
                if japanese and chinese:
                    # 两个部分都有，添加两行
                    processed_lines.append(f'[{timestamp}]{japanese}')
                    processed_lines.append(f'[{timestamp}]{chinese}')
                elif japanese:
                    # 只有原文，添加一行
                    processed_lines.append(f'[{timestamp}]{japanese}')
                elif chinese:
                    # 只有翻译，添加一行
                    processed_lines.append(f'[{timestamp}]{chinese}')
                else:
                    # 都没有，添加原始行
                    processed_lines.append(line)
        
        # 写入输出文件
        with open(output_file, 'w', encoding='utf-8') as f:
            for line in processed_lines:
                f.write(line + '\n')
        
        return output_file
    
    def _remove_copyright_notices(self, line: str) -> str:
        """
        删除版权声明和无关信息
        """
        # 需要删除的版权声明模式
        copyright_patterns = [
            r'QQ音乐享有本翻译作品的著作权',
            r'QQ音乐享有本翻译作品的著作权',
            r'享有本翻译作品的著作权',
            r'翻译作品著作权',
            r'版权所有',
            r'Copyright.*',
            r'©.*',
        ]
        
        for pattern in copyright_patterns:
            line = re.sub(pattern, '', line)
        
        # 清理多余的空格和标点
        line = re.sub(r'\s*-\s*$', '', line)  # 删除末尾的 " - "
        line = re.sub(r'\s+', ' ', line)  # 合并多个空格
        line = line.strip()
        
        return line
    
    def batch_process(self, input_dir: str, output_dir: str = None):
        """
        批量处理目录中的所有歌词文件
        """
        if output_dir is None:
            output_dir = input_dir
        
        # 支持多种歌词格式：.txt 和 .lrc
        # 排除已经分割过的文件（包含_split后缀的文件）
        input_files = [f for f in os.listdir(input_dir)
                      if (f.endswith('.txt') or f.endswith('.lrc'))
                      and '_split' not in f]
        
        results = []
        for filename in input_files:
            input_path = os.path.join(input_dir, filename)
            # 保持原始文件扩展名，只添加_split后缀
            base_name = os.path.splitext(filename)[0]
            file_ext = os.path.splitext(filename)[1]  # 保留原始扩展名 (.txt 或 .lrc)
            output_filename = f"{base_name}_split{file_ext}"
            output_path = os.path.join(output_dir, output_filename)
            
            try:
                self.process_file(input_path, output_path)
                results.append((filename, "成功", output_path))
            except Exception as e:
                results.append((filename, f"失败: {str(e)}", None))
        
        return results

def main():
    parser = argparse.ArgumentParser(description='歌词处理工具 - 自动分割日文和中文歌词')
    parser.add_argument('input', help='输入文件路径或目录')
    parser.add_argument('-o', '--output', help='输出文件路径或目录（可选）')
    parser.add_argument('-b', '--batch', action='store_true', help='批量处理目录中的所有歌词文件')
    
    args = parser.parse_args()
    
    processor = LyricProcessor()
    
    try:
        if args.batch or os.path.isdir(args.input):
            # 批量处理
            print(f"正在批量处理目录: {args.input}")
            results = processor.batch_process(args.input, args.output)
            
            print("\n处理结果:")
            for filename, status, output_path in results:
                print(f"  {filename}: {status}")
                if output_path:
                    print(f"    输出: {output_path}")
        else:
            # 单文件处理
            print(f"正在处理文件: {args.input}")
            output_file = processor.process_file(args.input, args.output)
            print(f"处理完成，输出文件: {output_file}")
    
    except Exception as e:
        print(f"错误: {e}")
        return 1
    
    return 0

if __name__ == "__main__":
    exit(main())