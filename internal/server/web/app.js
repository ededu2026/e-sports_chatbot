const chatForm = document.getElementById("chatForm");
const messageInput = document.getElementById("messageInput");
const chatMessages = document.getElementById("chatMessages");
const sendBtn = document.getElementById("sendBtn");
const newChatBtn = document.getElementById("newChatBtn");
const modelName = document.getElementById("modelName");
const healthPill = document.getElementById("healthPill");
const messageTemplate = document.getElementById("messageTemplate");
const conversationList = document.getElementById("conversationList");

let currentConversationId = null;
let currentMessages = [];
let conversations = [];

async function loadHealth() {
  try {
    const response = await fetch("/health");
    const data = await response.json();
    if (!data.ollama_reachable) {
      healthPill.textContent = "Ollama offline";
      modelName.textContent = `Start Ollama and pull ${data.model}`;
      return;
    }

    if (!data.model_available) {
      healthPill.textContent = "Model not installed";
      modelName.textContent = `Run: ollama pull ${data.model}`;
      return;
    }

    const storageText = data.storage_mode === "redis" ? "Redis cache on" : "Memory mode";
    healthPill.textContent = "Local model ready";
    modelName.textContent = `${data.model} via Ollama • ${storageText}`;
  } catch (error) {
    healthPill.textContent = "Backend unavailable";
    modelName.textContent = "Start the Go server and Ollama";
  }
}

function addMessage(role, content, meta = "", blocked = false, cached = false) {
  const node = messageTemplate.content.firstElementChild.cloneNode(true);
  node.classList.add(role);
  if (blocked) {
    node.classList.add("blocked");
  }
  if (cached) {
    node.classList.add("cached");
  }
  node.querySelector(".role").textContent = role === "user" ? "You" : "Assistant";
  node.querySelector(".meta").textContent = meta;
  node.querySelector(".bubble").textContent = content;
  chatMessages.appendChild(node);
  chatMessages.scrollTop = chatMessages.scrollHeight;
  return node;
}

function renderWelcome() {
  chatMessages.innerHTML = `
    <div class="welcome">
      <div class="welcome-badge">Local + Open Source</div>
      <h3>Ask about teams, tournaments, players, patches, metas, and match analysis.</h3>
      <p>Start with a greeting or jump straight into your eSports question.</p>
    </div>
  `;
}

function renderMessages(messages) {
  if (!messages.length) {
    renderWelcome();
    return;
  }

  chatMessages.innerHTML = "";
  messages.forEach((message) => {
    addMessage(message.role, message.content, "");
  });
}

function relativeTime(value) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "";
  }

  const diffMinutes = Math.max(0, Math.round((Date.now() - date.getTime()) / 60000));
  if (diffMinutes < 1) {
    return "just now";
  }
  if (diffMinutes < 60) {
    return `${diffMinutes}m ago`;
  }
  const diffHours = Math.round(diffMinutes / 60);
  if (diffHours < 24) {
    return `${diffHours}h ago`;
  }
  const diffDays = Math.round(diffHours / 24);
  return `${diffDays}d ago`;
}

function renderConversationList() {
  if (!conversations.length) {
    conversationList.innerHTML = `<p class="empty-sidebar">No conversations yet.</p>`;
    return;
  }

  conversationList.innerHTML = conversations
    .map(
      (conversation) => `
        <button class="conversation-item ${conversation.id === currentConversationId ? "active" : ""}" data-id="${conversation.id}">
          <span class="conversation-title">${escapeHTML(conversation.title)}</span>
          <span class="conversation-meta">${relativeTime(conversation.updated_at)}</span>
        </button>
      `
    )
    .join("");

  conversationList.querySelectorAll(".conversation-item").forEach((button) => {
    button.addEventListener("click", async () => {
      const id = button.dataset.id;
      await openConversation(id);
    });
  });
}

async function loadConversations() {
  try {
    const response = await fetch("/api/conversations?limit=12");
    const data = await response.json();
    conversations = data.conversations || [];
    renderConversationList();
  } catch (error) {
    conversationList.innerHTML = `<p class="empty-sidebar">Could not load conversations.</p>`;
  }
}

async function openConversation(id) {
  try {
    const response = await fetch(`/api/conversations/${id}`);
    if (!response.ok) {
      throw new Error("Conversation not found");
    }
    const conversation = await response.json();
    currentConversationId = conversation.id;
    currentMessages = conversation.messages || [];
    renderMessages(currentMessages);
    renderConversationList();
  } catch (error) {
    addMessage("assistant", error.message, "error", true);
  }
}

function startNewChat() {
  currentConversationId = null;
  currentMessages = [];
  renderWelcome();
  renderConversationList();
  messageInput.focus();
}

async function sendMessage(message) {
  sendBtn.disabled = true;
  if (!currentMessages.length) {
    chatMessages.innerHTML = "";
  }
  addMessage("user", message, "sent now");
  const assistantNode = addMessage("assistant", "", "thinking...");

  const history = [...currentMessages];

  try {
    const response = await fetch("/api/chat/stream", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        message,
        history,
        conversation_id: currentConversationId,
      }),
    });

    if (!response.ok) {
      throw new Error("Request failed");
    }

    const bubble = assistantNode.querySelector(".bubble");
    const meta = assistantNode.querySelector(".meta");
    const reader = response.body.getReader();
    const decoder = new TextDecoder();
    let buffer = "";
    let finalEvent = null;

    while (true) {
      const { value, done } = await reader.read();
      if (done) {
        break;
      }

      buffer += decoder.decode(value, { stream: true });
      const lines = buffer.split("\n");
      buffer = lines.pop() || "";

      for (const line of lines) {
        if (!line.trim()) {
          continue;
        }
        const event = JSON.parse(line);
        if (event.type === "start") {
          currentConversationId = event.conversation_id || currentConversationId;
          meta.textContent = event.model || "streaming";
          continue;
        }
        if (event.type === "token") {
          bubble.textContent += event.content || "";
          chatMessages.scrollTop = chatMessages.scrollHeight;
          continue;
        }
        if (event.type === "error") {
          throw new Error(event.error || "Streaming failed");
        }
        if (event.type === "done") {
          finalEvent = event;
        }
      }
    }

    if (!finalEvent) {
      throw new Error("No final response received");
    }

    currentConversationId = finalEvent.conversation_id || currentConversationId;
    currentMessages.push({ role: "user", content: message });
    currentMessages.push({ role: "assistant", content: finalEvent.answer });
    bubble.textContent = finalEvent.answer;
    meta.textContent = finalEvent.blocked ? finalEvent.reason : finalEvent.cached ? "cached response" : finalEvent.model;
    if (finalEvent.blocked) {
      assistantNode.classList.add("blocked");
    }
    if (finalEvent.cached) {
      assistantNode.classList.add("cached");
    }
    await loadConversations();
  } catch (error) {
    assistantNode.querySelector(".bubble").textContent = error.message;
    assistantNode.querySelector(".meta").textContent = "error";
    assistantNode.classList.add("blocked");
  } finally {
    sendBtn.disabled = false;
    messageInput.focus();
  }
}

function escapeHTML(text) {
  return text
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}

chatForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  const message = messageInput.value.trim();
  if (!message) {
    return;
  }

  messageInput.value = "";
  await sendMessage(message);
});

newChatBtn.addEventListener("click", () => {
  startNewChat();
});

messageInput.addEventListener("keydown", (event) => {
  if (event.key === "Enter" && !event.shiftKey) {
    event.preventDefault();
    chatForm.requestSubmit();
  }
});

startNewChat();
loadHealth();
loadConversations();
