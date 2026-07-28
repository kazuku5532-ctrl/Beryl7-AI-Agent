# Operations Manual & System Monitoring 🛠️

System: **Beryl 7 AI Agent**  
Document Version: **v1.0.0**

---

## 1. Daily Operations Commands

```bash
# Check Go Agent Service Status
ps | grep beryl7-agent

# View Real-time Telemetry Health
curl http://192.168.8.1:8888/api/health

# Scrape Prometheus Metrics
curl http://192.168.8.1:8888/metrics

# View Operational Logs
tail -f /tmp/beryl7.log
```
