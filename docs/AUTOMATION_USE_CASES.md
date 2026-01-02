# Automation & Machine-to-Machine Use Cases

While Vanish is excellent for human-to-human secret sharing, its core properties—**ephemerality**, **burn-on-read**, and **URL-based access**—make it a powerful tool for automated systems.

This document outlines three key patterns for using Vanish in automated workflows:
1.  **Secure Bootstrapping (Cloud-Init)**
2.  **CI/CD Pipeline Security**
3.  **Intrusion Detection (Honeytokens)**

---

## 1. Secure Bootstrapping (Cloud-Init)

**The Problem:**
When launching new cloud instances (EC2, Droplets, VMs), you often need to inject sensitive initial secrets (database passwords, API tokens, join keys). Placing these directly in "User Data" or environment variables is insecure because:
*   They remain visible in the cloud console forever.
*   They are often logged in plain text on the disk (`/var/log/cloud-init.log`).

**The Vanish Solution:**
Use Vanish as a "dead drop." The secret exists only long enough for the server to boot and consume it.

**Workflow:**
1.  **Orchestrator (Terraform/CI)** generates the secret.
2.  **Orchestrator** saves it to Vanish and gets a one-time URL.
3.  **Orchestrator** launches the instance, passing *only the URL* in the startup script.
4.  **Instance** boots, runs `curl` to fetch the secret, and configures itself.
5.  **Result:** The URL is now dead. Inspection of the cloud console reveals only an expired link.

**Example (Terraform + Cloud-Init):**

```bash
# 1. Create the secret using Vanish CLI
SECRET_URL=$(echo "my-super-secret-db-password" | vanish send --ttl 300 bootstrapper@internal)

# 2. Pass the URL to the instance
aws ec2 run-instances \
  --image-id ami-12345678 \
  --user-data "#!/bin/bash
    # Fetch the secret
    # Note: Requires a script/tool to decrypt the client-side hash if using full E2E
    # For server-side automation, use the API directly or the CLI tool
    
    DB_PASS=\$(curl -s -X GET $SECRET_URL/raw | jq -r .secret)
    
    # Configure Database
    sed -i \"s/PASSWORD_HERE/\$DB_PASS/" /etc/db_config.conf
    
    # URL is now burned. No trace left."
```

---

## 2. CI/CD Pipeline "Hot Potato"

**The Problem:**
In complex pipelines, secrets often need to be passed between independent steps or tools (e.g., from Terraform to Ansible, or Jenkins to a deployment script).
*   Passing secrets via `stdout` leaks them into build logs.
*   Writing them to files on shared runners creates security risks.

**The Vanish Solution:**
Use Vanish to pass a reference to the secret, rather than the secret itself.

**Workflow:**
1.  **Step A (Producer):** Generates a credential.
2.  **Step A:** Sends it to Vanish: `vanish send "secret" > /tmp/url`.
3.  **Output:** Step A prints `https://vanish.../m/abc` to the build logs. This is safe!
4.  **Step B (Consumer):** Reads the URL from the previous step's output or a shared file.
5.  **Step B:** Fetches the secret, uses it, and the message vanishes.

**Benefits:**
*   **Clean Logs:** Your CI/CD history contains only expired URLs, not actual API keys.
*   **Audit Trail:** Vanish logs *that* a secret was read, providing a timestamp for the handover.

---

## 3. Intrusion Detection (Honeytokens)

**The Problem:**
Detecting when an attacker has breached your network or file system can be difficult. You often don't know until it's too late.

**The Vanish Solution:**
Use a Vanish message as a "Canary" or "Honeytoken."

**Workflow:**
1.  Create a tempting secret message, e.g., "Production Root Database Password."
2.  Place the **Vanish URL** in a file that looks valuable but isn't used (e.g., `old_config.backup`, `notes.txt`, or a fake `.env` file).
3.  **The Trap:** Legitimate developers know this file is fake/old. An attacker scraping your repo or file system will find it and try to use it.
4.  **The Alert:** As soon as the attacker clicks the link, the message status changes to "Read" (burned).
5.  **Detection:** If you check the status (or if configured with webhooks), you get an immediate signal that someone is poking around where they shouldn't be.

**Example Setup:**
Create a file named `credentials_backup.txt` in your repo:
```text
# OLD PRODUCTION KEYS (DO NOT USE)
# Master DB: https://vanish.internal/m/9f8a7d...#key
```

If that link ever dies, you know you have a breach.

---

## Automation Best Practices

*   **Short TTLs:** For automated handovers (Cloud-Init, CI/CD), set the Time-To-Live (TTL) to the minimum necessary (e.g., 5-10 minutes). This minimizes the window of opportunity if the URL is intercepted.
*   **CLI Usage:** The `vanish-cli` is the preferred tool for these workflows as it handles encryption and API interaction natively in the shell.
*   **Error Handling:** Ensure your scripts handle the "Message Not Found" (404) error gracefully—this usually means the secret effectively "timed out" or was already consumed.
