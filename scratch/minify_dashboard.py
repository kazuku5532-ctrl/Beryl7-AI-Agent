import re

def minify_html(src_path, dst_path):
    with open(src_path, 'r', encoding='utf-8') as f:
        content = f.read()

    # Remove HTML comments except specific directives
    content = re.sub(r'<!--(?!\[if).*?-->', '', content, flags=re.DOTALL)
    
    # Compress multiple spaces / newlines in HTML structure
    lines = [line.strip() for line in content.splitlines() if line.strip()]
    minified = '\n'.join(lines)

    with open(dst_path, 'w', encoding='utf-8') as f:
        f.write(minified)

    print(f"Minified {src_path} ({len(content)} bytes) -> {dst_path} ({len(minified)} bytes)")

if __name__ == "__main__":
    minify_html("dashboard/Beryl7_Dashboard_Standalone.html", "dashboard/Beryl7_Dashboard_Standalone.min.html")
