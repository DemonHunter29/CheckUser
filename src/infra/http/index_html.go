package http

const HTML_CONTENT = `
<!DOCTYPE HTML>
<html lang="pt-br">

<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>CheckUser</title>

    <style>
        @import url('https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700&display=swap');

        * { box-sizing: border-box; }

        html, body {
            margin: 0;
            padding: 0;
            font-family: 'Inter', -apple-system, BlinkMacSystemFont, sans-serif;
            -webkit-font-smoothing: antialiased;
        }

        body {
            min-height: 100vh;
            display: flex;
            justify-content: center;
            align-items: center;
            background:
                radial-gradient(1200px 600px at 10% -10%, hsl(217 91% 60% / 0.08), transparent 60%),
                radial-gradient(1000px 700px at 50% 110%, hsl(220 70% 50% / 0.05), transparent 60%),
                hsl(220 40% 8%);
            color: hsl(220 25% 97%);
            padding: 20px;
        }

        .container {
            display: flex;
            flex-direction: column;
            align-items: stretch;
            width: 360px;
            max-width: 100%;
            padding: 28px;
            border-radius: 20px;
            background: linear-gradient(145deg, hsl(220 30% 16%) 0%, hsl(220 35% 12%) 100%);
            border: 1px solid hsl(220 22% 26%);
            box-shadow:
                0 16px 48px -8px hsl(220 40% 4% / 0.6),
                inset 0 1px 0 hsl(220 25% 97% / 0.03);
        }

        .title {
            display: flex;
            align-items: center;
            justify-content: center;
            gap: 10px;
            margin: 0 0 22px 0;
            font-size: 1.35rem;
            font-weight: 700;
            letter-spacing: -0.01em;
        }

        .title .dot {
            width: 8px;
            height: 8px;
            border-radius: 50%;
            background: hsl(217 91% 60%);
            box-shadow: 0 0 12px hsl(217 91% 60% / 0.6);
        }

        .connectionsArea {
            display: flex;
            align-items: center;
            justify-content: space-between;
            gap: 10px;
            padding: 12px 14px;
            margin-bottom: 18px;
            border-radius: 12px;
            background: hsl(220 38% 10%);
            border: 1px solid hsl(220 22% 26%);
        }

        .connectionsArea h4 {
            color: hsl(220 14% 65%);
            font-size: 0.72rem;
            font-weight: 500;
            letter-spacing: 0.08em;
            text-transform: uppercase;
            margin: 0;
        }

        .container-count {
            color: hsl(217 91% 75%);
            background: hsl(217 91% 60% / 0.12);
            border: 1px solid hsl(217 91% 60% / 0.3);
            padding: 4px 12px;
            border-radius: 999px;
            font-size: 0.85rem;
            font-weight: 600;
            font-variant-numeric: tabular-nums;
        }

        .user {
            display: flex;
            flex-direction: column;
            gap: 10px;
        }

        .search {
            width: 100%;
            padding: 12px 14px;
            background: hsl(220 38% 10%);
            outline: none;
            border: 1px solid hsl(220 22% 26%);
            border-radius: 12px;
            color: hsl(220 25% 97%);
            font-family: inherit;
            font-size: 0.9rem;
            transition: all 0.15s ease;
        }

        .search::placeholder { color: hsl(220 14% 50%); }

        .search:focus {
            border-color: hsl(217 91% 60% / 0.5);
            box-shadow: 0 0 0 3px hsl(217 91% 60% / 0.15);
            background: hsl(220 35% 12%);
        }

        .details {
            display: none;
            flex-direction: column;
            gap: 6px;
            padding: 10px;
            border-radius: 12px;
            background: hsl(220 38% 11% / 0.6);
            border: 1px solid hsl(220 22% 26%);
        }

        .detail {
            background: hsl(220 35% 14%);
            border: 1px solid hsl(220 22% 22%);
            border-radius: 8px;
            color: hsl(220 25% 97%);
            font-size: 0.82rem;
            text-align: center;
            padding: 8px 10px;
            font-variant-numeric: tabular-nums;
        }

        .detail:first-child {
            background: hsl(217 91% 60% / 0.08);
            border-color: hsl(217 91% 60% / 0.25);
            color: hsl(217 91% 80%);
            font-weight: 600;
        }

        .details-not-found {
            display: none;
            text-align: center;
            margin-top: 6px;
            padding: 10px;
            color: hsl(0 84% 70%);
            background: hsl(0 84% 60% / 0.08);
            border: 1px solid hsl(0 84% 60% / 0.25);
            border-radius: 10px;
            font-size: 0.85rem;
        }

        .device-redirect {
            margin-top: 18px;
            display: block;
            text-align: center;
            text-decoration: none;
            color: hsl(220 25% 97%);
            background: linear-gradient(135deg, hsl(217 91% 60%) 0%, hsl(224 76% 48%) 100%);
            border-radius: 12px;
            padding: 11px;
            font-size: 0.85rem;
            font-weight: 600;
            letter-spacing: 0.02em;
            box-shadow:
                0 1px 0 hsl(220 25% 97% / 0.1) inset,
                0 4px 16px hsl(217 91% 60% / 0.3);
            transition: all 0.15s ease;
        }

        .device-redirect:hover {
            transform: translateY(-1px);
            box-shadow:
                0 1px 0 hsl(220 25% 97% / 0.15) inset,
                0 6px 24px hsl(217 91% 60% / 0.5);
        }
    </style>
</head>

<body>
    <div class="container">
        <h1 class="title"><span class="dot"></span>CheckUser</h1>

        <div class="connectionsArea">
            <h4>Total de conexões</h4>
            <div class="container-count"><span id="total">00</span></div>
        </div>

        <div class="user">
            <input type="text" class="search" placeholder="Digite um nome de usuário…">
            <div class="details">
                <span class="detail"></span>
                <span class="detail"></span>
                <span class="detail"></span>
                <span class="detail"></span>
            </div>
            <div class="details-not-found">Usuário não encontrado</div>
        </div>

        <a class="device-redirect" href="/device">Ver Device IDs</a>
    </div>
    <script>
        let timeout = null

        const userNotFoundElement = document.querySelector('.details-not-found')
        const [nameElement, expiresAtElement, limitElement, connectionsElement] = document.querySelectorAll('.detail')
        const details = document.querySelector('.details')
        const search = document.querySelector('.search')

        const showUserNotFound = () => {
            userNotFoundElement.style.display = 'block'
            details.style.display = 'none'
        }
        const hideUserNotFound = () => {
            details.style.display = 'flex'
            userNotFoundElement.style.display = 'none'
        }
        const hideAll = () => {
            userNotFoundElement.style.display = 'none'
            details.style.display = 'none'
        }

        const searchHandler = value => setTimeout(async () => {
            if (!value) { hideAll(); return }
            const data = await fetch('/details/' + value).then(r => r.json())
            if (!data?.username) { showUserNotFound(); return }
            hideUserNotFound()
            nameElement.innerHTML = data.username ?? ''
            expiresAtElement.innerHTML = data.expires_at ?? ''
            limitElement.innerHTML = 'Limite: ' + (data.limit?.toString()?.padStart(2, '0') ?? '00')
            connectionsElement.innerHTML = 'Conexões: ' + (data.connections?.toString()?.padStart(2, '0') ?? '00')
        }, 500)

        search.addEventListener('keyup', e => {
            clearTimeout(timeout)
            timeout = searchHandler(e.target.value)
        })

        ;(async () => {
            const data = await fetch('/count').then(e => e.json())
            document.querySelector('#total').innerHTML = data.count.toString().padStart(2, '0')
        })()
    </script>
</body>

</html>
`
