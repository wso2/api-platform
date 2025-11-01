import React from 'react'
import ReactDOM from 'react-dom'
import WebSocketViewer from './components/WebsocketViewer/WebsocketViewer.tsx'

ReactDOM.render(
    <React.StrictMode>
        <WebSocketViewer/>
    </React.StrictMode>,
    document.getElementById('root')
)