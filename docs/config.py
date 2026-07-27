python3 -c "
import json, os
path = os.path.expanduser('~/.picoclaw/config.json')
with open(path) as f :
    config = json.load(f)
config['tools'] = {
    'web' : {
        'enabled' : True,
        'duckduckgo' : {
            'enabled' : True, 
            'max_results' : 5
        }
    }
}
config['channels'] = {
    'telegram' : {
        'enabled' : True,
        'token' : 'YOUR_TELEGRAM_BOT_TOKEN'
    }
}
with open(path, 'w') as f :
    json.dump(config, f, indent=2)
print('Telegram + web search added')
"