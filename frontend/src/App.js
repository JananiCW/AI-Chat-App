import { useState, useEffect } from "react";
import "./App.css";

const API = "http://localhost:3001";

function App() {
  const [chats, setChats] = useState([]);
  const [activeChatId, setActiveChatId] = useState(null);
  const [activeChat, setActiveChat] = useState(null);
  const [input, setInput] = useState("");
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    fetchChats();
  }, []);

  useEffect(() => {
    if (activeChatId) fetchChat(activeChatId);
  }, [activeChatId]);

  // 📥 Fetch all chats
  const fetchChats = async () => {
    try {
      const res = await fetch(`${API}/chats`);
      const data = await res.json();
      setChats(data || []);
    } catch (err) {
      console.error("Error fetching chats:", err);
    }
  };

  // 📥 Fetch single chat with messages
  const fetchChat = async (id) => {
    try {
      const res = await fetch(`${API}/chats/${id}`);
      const data = await res.json();
      setActiveChat(data);
    } catch (err) {
      console.error("Error fetching chat:", err);
    }
  };

  // ➕ Create new chat
  const createChat = async () => {
    try {
      const res = await fetch(`${API}/chats`, { method: "POST" });
      const data = await res.json();
      setChats([data, ...chats]);
      setActiveChatId(data.id);
    } catch (err) {
      console.error("Error creating chat:", err);
    }
  };

  // 💬 Send message
  const sendMessage = async () => {
    if (!input.trim() || !activeChatId || loading) return;

    const userText = input;
    setInput("");
    setLoading(true);

    // Optimistically add user message to UI
    setActiveChat((prev) => ({
      ...prev,
      messages: [...(prev?.messages || []), { role: "user", content: userText }],
    }));

    try {
      const res = await fetch(`${API}/chats/${activeChatId}/message`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ message: userText }),
      });

      const data = await res.json();

      if (data.error) {
        alert("Error: " + data.error);
      } else {
        // Add AI response
        setActiveChat((prev) => ({
          ...prev,
          messages: [...(prev?.messages || []), data.aiMessage],
        }));
        fetchChats(); // update sidebar titles
      }
    } catch (err) {
      alert("Failed to connect to server!");
    }

    setLoading(false);
  };

  // 🗑️ Delete chat
  const deleteChat = async (id) => {
    try {
      await fetch(`${API}/chats/${id}`, { method: "DELETE" });
      setChats(chats.filter((c) => c.id !== id));
      if (activeChatId === id) {
        setActiveChatId(null);
        setActiveChat(null);
      }
    } catch (err) {
      console.error("Error deleting chat:", err);
    }
  };

  return (
    <div className="app">
      {/* ── Sidebar ── */}
      <div className="sidebar">
        <div className="sidebar-header">
          <h2>💬 AI Chat</h2>
          <button className="new-chat-btn" onClick={createChat}>
            + New Chat
          </button>
        </div>

        <div className="chat-list">
          {chats.length === 0 && (
            <p className="no-chats">No chats yet. Start one!</p>
          )}
          {chats.map((chat) => (
            <div
              key={chat.id}
              className={`chat-item ${activeChatId === chat.id ? "active" : ""}`}
              onClick={() => setActiveChatId(chat.id)}
            >
              <span className="chat-title">{chat.title || "New Chat"}</span>
              <button
                className="delete-chat-btn"
                onClick={(e) => {
                  e.stopPropagation();
                  deleteChat(chat.id);
                }}
              >
                🗑️
              </button>
            </div>
          ))}
        </div>
      </div>

      {/* ── Main Chat Area ── */}
      <div className="main">
        {!activeChatId ? (
          <div className="welcome">
            <h1>🤖 AI Chat App</h1>
            <p>Start a new conversation with AI!</p>
            <button className="start-btn" onClick={createChat}>
              + Start New Chat
            </button>
          </div>
        ) : (
          <>
            {/* Messages */}
            <div className="messages">
              {activeChat?.messages?.length === 0 && (
                <p className="empty-chat">Send a message to start chatting!</p>
              )}
              {activeChat?.messages?.map((msg, i) => (
                <div key={i} className={`message ${msg.role}`}>
                  <div className="message-avatar">
                    {msg.role === "user" ? "👤" : "🤖"}
                  </div>
                  <div className="message-content">
                    <p>{msg.content}</p>
                  </div>
                </div>
              ))}
              {loading && (
                <div className="message ai">
                  <div className="message-avatar">🤖</div>
                  <div className="message-content typing">
                    <span></span><span></span><span></span>
                  </div>
                </div>
              )}
            </div>

            {/* Input */}
            <div className="input-area">
              <input
                type="text"
                placeholder="Type your message..."
                value={input}
                onChange={(e) => setInput(e.target.value)}
                onKeyDown={(e) => e.key === "Enter" && sendMessage()}
                disabled={loading}
              />
              <button
                className="send-btn"
                onClick={sendMessage}
                disabled={loading || !input.trim()}
              >
                {loading ? "..." : "➤"}
              </button>
            </div>
          </>
        )}
      </div>
    </div>
  );
}

export default App;