import hashlib
import os

def sign_binary():
    target_bin = "go-agent/beryl7-agent"
    if not os.path.exists(target_bin):
        print(f"[Warn] Binary {target_bin} not built yet. Generating checksum placeholder.")
        return

    with open(target_bin, "rb") as f:
        digest = hashlib.sha256(f.read()).hexdigest()

    checksum_file = "go-agent/beryl7-agent.sha256"
    with open(checksum_file, "w", encoding="utf-8") as f:
        f.write(f"{digest}  beryl7-agent\n")
    print(f"Generated SHA256 Checksum ({digest}) at {checksum_file}")

if __name__ == "__main__":
    sign_binary()
