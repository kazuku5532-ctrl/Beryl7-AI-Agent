# Hardware & OS Compatibility Matrix 🖥️

System: **Beryl 7 AI Agent**  
Document Version: **v1.0.0**

---

## 1. Operating System & Firmware Support

| OS / Firmware | Version | Status | Notes |
| :--- | :--- | :--- | :--- |
| **OpenWrt** | `24.10.x` | 🟢 Fully Certified | Target Production Release |
| **OpenWrt** | `23.05.x` | 🟢 Fully Certified | Native support for ubus/UCI |
| **GL.iNet Firmware** | `v4.x` | 🟢 Fully Certified | Based on OpenWrt 23.05/24.10 |
| **Ubuntu / Debian** | `22.04 / 24.04` | 🟡 Controller Only | Runs Python REST server & Dashboard |

---

## 2. Hardware Architecture Support

| Device Hardware | Architecture | CPU | RAM | Support Tier |
| :--- | :--- | :--- | :--- | :--- |
| **GL.iNet Beryl 7 (GL-MT3600BE)** | `linux/arm64` | MediaTek Filogic 820 | 512 MB DDR4 | **Tier 1 (Primary)** |
| **GL.iNet Flint 2 (GL-MT6000)** | `linux/arm64` | MediaTek Filogic 830 | 1 GB DDR4 | **Tier 1 (Supported)** |
| **Raspberry Pi 4 / 5** | `linux/arm64` | Broadcom BCM2711/2712 | 2-8 GB | **Tier 1 (Supported)** |
| **Banana Pi BPI-R3 / R4** | `linux/arm64` | MediaTek Filogic 830/880 | 2-4 GB | **Tier 1 (Supported)** |
| **NanoPi R4S / R6S** | `linux/arm64` | Rockchip RK3399/RK3588 | 1-8 GB | **Tier 1 (Supported)** |
