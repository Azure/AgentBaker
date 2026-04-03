---
name: save-markdown-to-disk
description: >
  Save markdown content to a file on disk without bash HEREDOC corruption.
  Use this skill whenever you need to write markdown, code, or any multi-line
  text containing backticks, dollar signs, single quotes, or other shell
  metacharacters to a file.
allowed-tools: create, python3, shell
---

# Save Markdown to Disk

## Problem

Writing markdown to files using bash HEREDOC (`cat << 'EOF'`) breaks when content contains:
- Backticks (`` ` ``) — interpreted as command substitution even in some HEREDOC forms
- `$variable` — interpreted as shell expansion in unquoted HEREDOCs
- The HEREDOC delimiter appearing in the content itself
- Nested quotes and backslashes causing silent corruption

This is the #1 cause of garbled reports when agents write files via shell.

## Solution: use the create tool

You should use the `create` tool to write markdown files, which handles all escaping and encoding issues for you. This should be the first thing you try when you need to save markdown content to disk.

## Solution 2: Use a quoted HEREDOC with a unique delimiter

You can use a quoted HEREDOC with a unique delimiter to avoid shell expansion. However, this method can still fail if the content contains the delimiter or certain combinations of characters. Sometimes the bash tool will block this approach if there appears to be dangerous shell commands in the document. Here's how you can do it:

```bash
cat << 'UNIQUE_DELIMITER' > final_markdown.md
<your_markdown_content_here>
UNIQUE_DELIMITER
```

## Solution 3: base64 encode the content

If you don't have the create tool available, you can write base64 encoded content to avoid shell escaping issues. However, due to the issues mentioned above, you must base64 encode the content in the HEREDOC and decode it when writing to the file. You must base64 encode the content yourself - do not rely on external tools. Here's how you extract the base64 content and write it to a file:

```bash
# Base64 encode the markdown content and write it to a file
echo "<your_base64_encoded_content_here>" | base64 --decode encoded_content.txt > final_markdown.md
```
