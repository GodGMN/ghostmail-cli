# Ghostmail CLI - Agent Skill Guide

This guide provides instructions for using ghostmail-cli as an agent for email automation tasks.

## Overview

Ghostmail CLI is a command-line email client that supports SMTP (sending) and IMAP (reading). It uses environment variables for configuration and supports JSON output for easy parsing.

## Quick Reference

| Task | Command |
|------|---------|
| Send alert email | `ghostmail send -t admin@example.com -s "Alert" -b "Message"` |
| Check unread emails | `ghostmail inbox --unread --json` |
| Read specific email | `ghostmail read --uid 12345 --json` |
| List inbox | `ghostmail inbox --limit 10 --json` |
| Send with attachment | `ghostmail send -t user@example.com -s "Doc" -b "See attached" -a file.pdf` |

## Environment Setup

Before using ghostmail-cli, ensure these environment variables are set:

```bash
# Required for sending emails
export GHOSTMAIL_SMTP_HOST="smtp.gmail.com"
export GHOSTMAIL_SMTP_PORT="587"
export GHOSTMAIL_SMTP_USERNAME="your-email@gmail.com"
export GHOSTMAIL_SMTP_PASSWORD="your-app-password"
export GHOSTMAIL_SMTP_FROM="your-email@gmail.com"

# Required for reading emails
export GHOSTMAIL_IMAP_HOST="imap.gmail.com"
export GHOSTMAIL_IMAP_PORT="993"
export GHOSTMAIL_IMAP_USERNAME="your-email@gmail.com"
export GHOSTMAIL_IMAP_PASSWORD="your-app-password"
```

### Gmail Setup

For Gmail accounts, use an **App Password** instead of your regular password:
1. Enable 2-Factor Authentication
2. Generate an App Password at: https://myaccount.google.com/apppasswords
3. Use the 16-character password in `GHOSTMAIL_SMTP_PASSWORD` and `GHOSTMAIL_IMAP_PASSWORD`

### Verify Configuration

```bash
ghostmail config check
```

## Common Patterns

### 1. Send Alert Email

Send a simple notification email:

```bash
ghostmail send \
  --to "admin@example.com" \
  --subject "Server Alert" \
  --body "Disk usage is at 90%"
```

**JSON version (for scripting):**

```bash
result=$(ghostmail send \
  --to "admin@example.com" \
  --subject "Server Alert" \
  --body "Disk usage is at 90%" \
  --json)

# Parse result
if echo "$result" | jq -e '.success' > /dev/null; then
  echo "Email sent successfully"
else
  echo "Failed: $(echo "$result" | jq -r '.error')"
fi
```

### 2. Read Unread Emails

Get all unread emails and process them:

```bash
# Get unread emails as JSON
ghostmail inbox --unread --json
```

**Example output:**

```json
{
  "success": true,
  "messages": [
    {
      "uid": 12345,
      "subject": "Important Update",
      "from": "sender@example.com",
      "date": "2024-01-15T10:30:00Z",
      "flags": []
    }
  ],
  "total": 1
}
```

**Parse with jq:**

```bash
# Get subjects of unread emails
ghostmail inbox --unread --json | jq -r '.messages[].subject'

# Count unread emails
ghostmail inbox --unread --json | jq '.messages | length'

# Get first unread UID
ghostmail inbox --unread --json | jq -r '.messages[0].uid'
```

### 3. Read Full Email Content

Read a specific email by UID:

```bash
ghostmail read --uid 12345 --json
```

**Example output:**

```json
{
  "success": true,
  "message": {
    "uid": 12345,
    "subject": "Important Update",
    "from": "sender@example.com",
    "to": ["recipient@example.com"],
    "cc": [],
    "date": "2024-01-15T10:30:00Z",
    "body": "Full email body content here...",
    "body_preview": "Preview text...",
    "flags": ["\\Seen"]
  }
}
```

### 4. Send Email with CC/BCC

```bash
ghostmail send \
  --to "primary@example.com" \
  --cc "cc1@example.com" \
  --cc "cc2@example.com" \
  --bcc "hidden@example.com" \
  --subject "Meeting Notes" \
  --body "Please review the attached notes"
```

### 5. Send HTML Email

```bash
# From HTML file
ghostmail send \
  --to "user@example.com" \
  --subject "Newsletter" \
  --html-file newsletter.html \
  --body "Plain text fallback"

# Or inline (useful in scripts)
echo "<h1>Hello</h1><p>World</p>" > /tmp/body.html
ghostmail send \
  --to "user@example.com" \
  --subject "HTML Test" \
  --html-file /tmp/body.html \
  --body "Plain text fallback"
```

### 6. Send Email with Attachments

```bash
ghostmail send \
  --to "recipient@example.com" \
  --subject "Documents" \
  --body "Please find attached" \
  --attach report.pdf \
  --attach data.csv
```

**Limitations:**
- Maximum 5 attachments per email
- Maximum 10MB per attachment

### 7. Reply to Email (Threading)

To reply to an email and maintain the thread:

```bash
# First, get the original message ID
original_msg=$(ghostmail read --uid 12345 --json)
message_id=$(echo "$original_msg" | jq -r '.message.headers["Message-ID"]')

# Send reply with In-Reply-To header
ghostmail send \
  --to "sender@example.com" \
  --subject "Re: Original Subject" \
  --body "My reply text" \
  --in-reply-to "$message_id"
```

### 8. Process Latest Emails

Process the 10 most recent emails:

```bash
# Get latest 10 emails
emails=$(ghostmail inbox --limit 10 --json)

# Loop through and process
for uid in $(echo "$emails" | jq -r '.messages[].uid'); do
  email=$(ghostmail read --uid "$uid" --json)
  subject=$(echo "$email" | jq -r '.message.subject')
  from=$(echo "$email" | jq -r '.message.from')
  
  echo "Processing: $subject from $from"
  # Add your processing logic here
done
```

### 9. Monitor for Specific Emails

Check for emails from a specific sender:

```bash
ghostmail inbox --json | jq '.messages[] | select(.from | contains("specific-sender"))'
```

### 10. Send Batch Notifications

Send the same message to multiple recipients:

```bash
recipients=("user1@example.com" "user2@example.com" "user3@example.com")

for recipient in "${recipients[@]}"; do
  ghostmail send \
    --to "$recipient" \
    --subject "System Maintenance" \
    --body "The system will be down for maintenance tonight." \
    --json
done
```

## JSON Output Reference

All commands support `--json` for machine-readable output.

### Success Response

```json
{
  "success": true,
  "message": "..."
}
```

### Error Response

```json
{
  "success": false,
  "error": "Error message here"
}
```

### Inbox Response

```json
{
  "success": true,
  "messages": [
    {
      "uid": 12345,
      "seq_num": 100,
      "subject": "Subject line",
      "from": "Sender <sender@example.com>",
      "to": ["recipient@example.com"],
      "cc": [],
      "bcc": [],
      "date": "2024-01-15T10:30:00Z",
      "body_preview": "Preview text...",
      "flags": ["\\Seen"]
    }
  ],
  "total": 1
}
```

### Read Response

```json
{
  "success": true,
  "message": {
    "uid": 12345,
    "subject": "Subject line",
    "from": "Sender <sender@example.com>",
    "to": ["recipient@example.com"],
    "date": "2024-01-15T10:30:00Z",
    "body": "Full body content...",
    "body_preview": "Preview...",
    "attachments": [
      {
        "filename": "document.pdf",
        "content_type": "application/pdf",
        "size": 12345
      }
    ],
    "flags": ["\\Seen"]
  }
}
```

## Error Handling

Always check for errors in JSON responses:

```bash
result=$(ghostmail send -t user@example.com -s "Test" -b "Body" --json)

if ! echo "$result" | jq -e '.success' > /dev/null 2>&1; then
  error=$(echo "$result" | jq -r '.error')
  echo "Failed to send email: $error"
  exit 1
fi
```

## Python Integration Example

```python
import subprocess
import json
import os

# Set up environment
os.environ['GHOSTMAIL_SMTP_HOST'] = 'smtp.gmail.com'
os.environ['GHOSTMAIL_SMTP_USERNAME'] = 'bot@example.com'
os.environ['GHOSTMAIL_SMTP_PASSWORD'] = 'app-password'

def send_email(to, subject, body):
    """Send an email using ghostmail-cli."""
    result = subprocess.run(
        ['ghostmail', 'send', '--to', to, '--subject', subject, '--body', body, '--json'],
        capture_output=True,
        text=True
    )
    
    response = json.loads(result.stdout)
    if not response.get('success'):
        raise Exception(f"Failed to send email: {response.get('error')}")
    
    return response

def get_unread_emails():
    """Get all unread emails."""
    result = subprocess.run(
        ['ghostmail', 'inbox', '--unread', '--json'],
        capture_output=True,
        text=True
    )
    
    response = json.loads(result.stdout)
    if not response.get('success'):
        raise Exception(f"Failed to get emails: {response.get('error')}")
    
    return response.get('messages', [])

def read_email(uid):
    """Read a specific email by UID."""
    result = subprocess.run(
        ['ghostmail', 'read', '--uid', str(uid), '--json'],
        capture_output=True,
        text=True
    )
    
    response = json.loads(result.stdout)
    if not response.get('success'):
        raise Exception(f"Failed to read email: {response.get('error')}")
    
    return response.get('message')

# Example usage
if __name__ == "__main__":
    # Send an alert
    send_email("admin@example.com", "System Alert", "CPU usage is high")
    
    # Check for new emails
    unread = get_unread_emails()
    for email in unread:
        print(f"New email from {email['from']}: {email['subject']}")
```

## Security Best Practices

1. **Use App Passwords**: For Gmail and similar providers, never use your main account password
2. **Environment Variables**: Store credentials in environment variables, never in code
3. **Dedicated Account**: Consider using a dedicated email account for automation
4. **Least Privilege**: Use minimal required permissions for the email account
5. **No Logging**: Be careful not to log commands that might include sensitive data

## Troubleshooting

### "SMTP host is required"
- Environment variables not set
- Run `ghostmail config check` to verify

### "Authentication failed"
- Check username and password
- For Gmail: ensure you're using an App Password
- Verify 2FA is enabled for App Passwords

### "inbox returns no messages"
- Check IMAP configuration
- Verify IMAP is enabled in email provider settings
- Try with `--verbose` flag for details

### "attachment too large"
- Maximum attachment size is 10MB per file
- Maximum 5 attachments per email
- Use file sharing services for larger files

## Resources

- **Help**: `ghostmail --help` or `ghostmail <command> --help`
- **Config Example**: `ghostmail config example`
- **Config Check**: `ghostmail config check`
