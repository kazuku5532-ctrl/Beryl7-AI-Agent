# Network Telemetry KPIs & Definitions 📡

System: **Beryl 7 AI Agent**  
Document Version: **v1.0.0**

---

## 1. Network Telemetry Metric Definitions

| Indicator | Metric Source | Definition | Normal Threshold |
| :--- | :--- | :--- | :--- |
| **Conntrack Sessions** | `/proc/sys/net/netfilter/nf_conntrack_count` | Number of active NAT/firewall connections | < 30,000 |
| **ARP Table Entries** | `/proc/net/arp` | Number of active connected LAN/Wi-Fi devices | < 254 |
| **Interface PER** | `/proc/net/dev` | Packet Error Rate = Errors / (Packets + Errors) | < 0.01% |
| **Interface Drops** | `/proc/net/dev` | Dropped rx/tx packets counter | < 0.1% |
| **DNS Probe RTT** | `net.DialTimeout("tcp", "1.1.1.1:53")` | Latency to cloud DNS resolver | < 50.0 ms |
