(function () {
  "use strict";

  const pageRoot = (() => {
    if (typeof window.API_ROOT === "string" && window.API_ROOT) {
      return window.API_ROOT.replace(/\/$/, "");
    }
    const match = window.location.pathname.match(/^(.*?\/music)(?:\/|$)/);
    return match ? match[1] : "/music";
  })();

  let syncQueued = false;

  function icon(name) {
    return `<i class="fa-solid ${name}" aria-hidden="true"></i>`;
  }

  function navigate(path) {
    document.body.classList.remove("sidebar-open");
    const url = path.startsWith("http") ? path : pageRoot + path;
    if (typeof window.navigateTo === "function") {
      window.navigateTo(url);
      return;
    }
    window.location.href = url;
  }

  function callOrNavigate(functionName, fallbackPath) {
    document.body.classList.remove("sidebar-open");
    if (typeof window[functionName] === "function") {
      window[functionName]();
      return;
    }
    navigate(fallbackPath);
  }

  function navButton(id, iconName, label) {
    return `<button type="button" class="app-nav-item" data-theme-nav="${id}" title="${label}">${icon(iconName)}<span class="app-nav-label">${label}</span></button>`;
  }

  function createShell() {
    if (!document.querySelector(".container")) return;

    if (!document.getElementById("music-client-sidebar")) {
      const sidebar = document.createElement("aside");
      sidebar.id = "music-client-sidebar";
      sidebar.className = "app-sidebar";
      sidebar.setAttribute("aria-label", "主导航");
      sidebar.innerHTML = `
        <a class="app-brand" href="${pageRoot}" data-theme-home title="返回发现音乐">
          <img src="${pageRoot}/icon.png" alt="">
          <span class="app-brand-copy">
            <span class="app-brand-name">Music DL</span>
            <span class="app-brand-version">Web Player</span>
          </span>
        </a>
        <nav class="app-nav">
          <div class="app-nav-section">在线音乐</div>
          ${navButton("home", "fa-compass", "发现音乐")}
          ${navButton("guess", "fa-headphones-simple", "猜你喜欢")}
          ${navButton("recommend", "fa-fire", "每日推荐")}
          ${navButton("categories", "fa-layer-group", "歌单分类")}
          ${navButton("mine", "fa-heart", "我的歌单")}
          <div class="app-nav-section">我的音乐</div>
          ${navButton("collections", "fa-folder-open", "本地歌单")}
          ${navButton("local", "fa-music", "本地音乐")}
        </nav>
        <div class="app-sidebar-spacer"></div>
        <div class="app-sidebar-tools" id="music-client-tools" aria-label="快捷工具"></div>`;
      document.body.prepend(sidebar);

      sidebar.querySelector("[data-theme-home]").addEventListener("click", (event) => {
        event.preventDefault();
        navigate("");
      });
      sidebar.querySelector('[data-theme-nav="home"]').addEventListener("click", () => navigate(""));
      sidebar.querySelector('[data-theme-nav="guess"]').addEventListener("click", () => callOrNavigate("goToGuessYouLike", "/guess_you_like"));
      sidebar.querySelector('[data-theme-nav="recommend"]').addEventListener("click", () => callOrNavigate("goToRecommend", "/recommend"));
      sidebar.querySelector('[data-theme-nav="categories"]').addEventListener("click", () => callOrNavigate("goToPlaylistCategories", "/playlist_categories"));
      sidebar.querySelector('[data-theme-nav="mine"]').addEventListener("click", () => callOrNavigate("goToUserPlaylists", "/user_playlists"));
      sidebar.querySelector('[data-theme-nav="collections"]').addEventListener("click", () => callOrNavigate("openCollectionManager", "/my_collections"));
      sidebar.querySelector('[data-theme-nav="local"]').addEventListener("click", () => callOrNavigate("openLocalMusicPage", "/local_music_page"));
    }

    if (!document.getElementById("music-client-topbar")) {
      const topbar = document.createElement("header");
      topbar.id = "music-client-topbar";
      topbar.className = "app-topbar";
      topbar.innerHTML = `
        <button type="button" class="app-menu-toggle" title="打开导航" aria-label="打开导航">${icon("fa-bars")}</button>
        <div class="app-history-controls">
          <button type="button" class="app-icon-button" data-theme-back title="后退" aria-label="后退">${icon("fa-chevron-left")}</button>
          <button type="button" class="app-icon-button" data-theme-forward title="前进" aria-label="前进">${icon("fa-chevron-right")}</button>
        </div>
        <span class="app-mobile-brand">Music DL</span>
        <div class="app-topbar-search" id="music-client-search"></div>
        <div class="app-page-title" id="music-client-page-title">发现音乐</div>`;
      document.body.prepend(topbar);
      topbar.querySelector("[data-theme-back]").addEventListener("click", () => window.history.back());
      topbar.querySelector("[data-theme-forward]").addEventListener("click", () => window.history.forward());
      topbar.querySelector(".app-menu-toggle").addEventListener("click", () => document.body.classList.toggle("sidebar-open"));
    }

    if (!document.getElementById("music-client-scrim")) {
      const scrim = document.createElement("div");
      scrim.id = "music-client-scrim";
      scrim.className = "app-sidebar-scrim";
      scrim.addEventListener("click", () => document.body.classList.remove("sidebar-open"));
      document.body.append(scrim);
    }
  }

  function decorateToolbar() {
    const toolbar = document.querySelector(".right-toolbar");
    const host = document.getElementById("music-client-tools");
    if (!toolbar || !host) return;
    if (toolbar.parentElement !== host) host.append(toolbar);

    toolbar.querySelectorAll(".rt-btn").forEach((button) => {
      button.dataset.themeLabel = button.getAttribute("title") || button.getAttribute("aria-label") || "工具";
      if (button.getAttribute("title") === "本地音乐") {
        button.classList.add("theme-duplicate-tool");
      }
    });
    const authForm = toolbar.querySelector("#auth-float-form");
    if (authForm) {
      authForm.dataset.themeLabel = authForm.getAttribute("title") || "登录";
    }
  }

  function moveSearch() {
    const host = document.getElementById("music-client-search");
    const inputGroup = document.querySelector(".container .search-box .input-group");
    if (!host || !inputGroup) return;

    inputGroup.querySelectorAll("input, button").forEach((control) => {
      control.setAttribute("form", "search-form");
    });
    host.replaceChildren(inputGroup);
  }

  function markLegacyNavigation() {
    const searchBox = document.querySelector(".container .search-box");
    if (!searchBox) return;
    const quickButton = Array.from(searchBox.querySelectorAll("button")).find((button) => {
      const action = button.getAttribute("onclick") || "";
      return action.includes("goToRecommend") || action.includes("openCollectionManager");
    });
    if (quickButton && quickButton.parentElement) {
      quickButton.parentElement.classList.add("legacy-quick-nav");
    }
  }

  function currentSection() {
    const path = window.location.pathname;
    if (path.includes("/guess_you_like")) return "guess";
    if (path.includes("/recommend")) return "recommend";
    if (path.includes("/playlist_categories") || path.includes("/category_playlists")) return "categories";
    if (path.includes("/user_playlists")) return "mine";
    if (path.includes("/my_collections") || path.includes("/collection")) return "collections";
    if (path.includes("/local_music_page")) return "local";
    return "home";
  }

  function pageTitle() {
    const path = window.location.pathname;
    const section = currentSection();
    const titleMap = {
      home: new URLSearchParams(window.location.search).has("q") ? "搜索结果" : "发现音乐",
      guess: "猜你喜欢",
      recommend: "每日推荐",
      categories: "歌单分类",
      mine: "我的歌单",
      collections: "本地歌单",
      local: "本地音乐",
    };
    if (path === pageRoot + "/album" || path === pageRoot + "/album_jump") return "专辑详情";
    if (path === pageRoot + "/playlist") return "歌单详情";
    return titleMap[section];
  }

  function updateNavigation() {
    const active = currentSection();
    document.querySelectorAll("[data-theme-nav]").forEach((button) => {
      const isActive = button.dataset.themeNav === active;
      button.classList.toggle("is-active", isActive);
      if (isActive) button.setAttribute("aria-current", "page");
      else button.removeAttribute("aria-current");
    });
    const title = document.getElementById("music-client-page-title");
    if (title) title.textContent = pageTitle();
  }

  function removeDecoration() {
    const widget = document.getElementById("sakana-container");
    if (widget) widget.remove();
  }

  function syncTheme() {
    syncQueued = false;
    document.documentElement.classList.add("music-client-theme-root");
    document.body.classList.add("music-client-theme");
    if (document.body.classList.contains("auth-page")) return;
    createShell();
    moveSearch();
    markLegacyNavigation();
    decorateToolbar();
    updateNavigation();
    removeDecoration();
  }

  function queueSync() {
    if (syncQueued) return;
    syncQueued = true;
    window.requestAnimationFrame(syncTheme);
  }

  syncTheme();

  const observer = new MutationObserver(queueSync);
  observer.observe(document.body, {
    childList: true,
    subtree: true,
    attributes: true,
    attributeFilter: ["title"],
  });
  window.addEventListener("popstate", queueSync);
  window.addEventListener("resize", () => {
    if (window.innerWidth > 768) document.body.classList.remove("sidebar-open");
  });
})();
