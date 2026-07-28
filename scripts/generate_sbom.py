import json
import time

def generate_sbom():
    sbom_data = {
        "spdxVersion": "SPDX-2.3",
        "dataLicense": "CC0-1.0",
        "SPDXID": "SPDXRef-DOCUMENT",
        "name": "Beryl7-AI-Agent-SBOM",
        "nameSpace": "https://github.com/kazuku5532-ctrl/Beryl7-AI-Agent",
        "created": time.strftime("%Y-%m-%dT%H:%M:%SZ"),
        "packages": [
            {
                "name": "beryl7-agent",
                "versionInfo": "15.3.0",
                "downloadLocation": "https://github.com/kazuku5532-ctrl/Beryl7-AI-Agent/releases",
                "licenseConcluded": "MIT"
            },
            {
                "name": "modernc.org/sqlite",
                "versionInfo": "v1.34.0",
                "licenseConcluded": "BSD-3-Clause"
            }
        ]
    }
    
    out_path = "docs/sbom.spdx.json"
    with open(out_path, "w", encoding="utf-8") as f:
        json.dump(sbom_data, f, indent=2)
    print(f"Generated SBOM SPDX file at {out_path}")

if __name__ == "__main__":
    generate_sbom()
