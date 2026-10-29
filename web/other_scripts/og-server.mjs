import { createServer } from "node:http";
import { readFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import satori from "satori";
import sharp from "sharp";

const PORT = 3001;
const SITE_NAME =
  "uploader.jp保全プロジェクト - 全国もちもちうどん連盟";

const FONT_PATH = fileURLToPath(
  new URL(
    "../node_modules/@fontsource/noto-sans-jp/files/noto-sans-jp-japanese-400-normal.woff2",
    import.meta.url,
  ),
);

const font = await readFile(FONT_PATH);
const cache = new Map();
const MAX_CACHE = 100;

// キャッシュ件数とは独立して、生成負荷を制限する。
const MAX_CONCURRENT_GENERATIONS = 4;
const MAX_QUEUED_GENERATIONS = 32;
let activeGenerations = 0;
const generationQueue = [];
const inFlight = new Map();

function acquireGenerationSlot() {
  if (activeGenerations < MAX_CONCURRENT_GENERATIONS) {
    activeGenerations++;
    return Promise.resolve();
  }

  if (generationQueue.length >= MAX_QUEUED_GENERATIONS) {
    const error = new Error("OGP generation queue is full");
    error.statusCode = 503;
    return Promise.reject(error);
  }

  return new Promise((resolve) => {
    generationQueue.push(resolve);
  });
}

function releaseGenerationSlot() {
  const next = generationQueue.shift();

  if (next) {
    // 待機中の処理に枠を引き継ぐ。
    next();
    return;
  }

  activeGenerations--;
}

async function getOrGenerate(key, title, description) {
  const cached = cache.get(key);
  if (cached) return cached;

  const existing = inFlight.get(key);
  if (existing) return existing;

  let promise;
  promise = (async () => {
    await acquireGenerationSlot();

    try {
      const png = await generate(title, description);

      if (cache.size >= MAX_CACHE) {
        cache.delete(cache.keys().next().value);
      }

      cache.set(key, png);
      return png;
    } finally {
      releaseGenerationSlot();
    }
  })().finally(() => {
    if (inFlight.get(key) === promise) {
      inFlight.delete(key);
    }
  });

  inFlight.set(key, promise);
  return promise;
}

function clean(value, fallback, maxLength) {
  if (!value) return fallback;

  return [...value.replace(/[\u0000-\u001f\u007f]/g, " ").trim()]
    .slice(0, maxLength)
    .join("") || fallback;
}

function element(style, children) {
  return {
    type: "div",
    props: {
      style: {
        display: "flex",
        ...style,
      },
      children,
    },
  };
}

async function generate(title, description) {
  const tree = element(
    {
      position: "relative",
      width: "1200px",
      height: "630px",
      overflow: "hidden",
      background:
        "linear-gradient(106deg, #e8f1ff 0%, #f5f8ff 65%, #f5f4df 65%, #fbf9e9 100%)",
      fontFamily: "Noto Sans JP",
      color: "#111111",
    },
    [
      // 左上のアクセントライン
      element(
        {
          position: "absolute",
          left: "70px",
          top: "76px",
          width: "120px",
          height: "26px",
          flexDirection: "column",
          gap: "6px",
        },
        [
          element({
            width: "120px",
            height: "10px",
            backgroundColor: "#b39500",
          }),
          element({
            width: "70px",
            height: "10px",
            backgroundColor: "#b39500",
          }),
        ],
      ),

      // ページタイトル：元画像に合わせて左上に大きく配置
      element(
        {
          position: "absolute",
          left: "80px",
          top: "138px",
          width: "1040px",
          height: "190px",
          fontSize: "64px",
          fontWeight: 700,
          lineHeight: 1.35,
          overflow: "hidden",
          wordBreak: "break-word",
        },
        title,
      ),

      // ページ説明文
      element(
        {
          position: "absolute",
          left: "82px",
          top: "416px",
          width: "1030px",
          height: "84px",
          fontSize: "28px",
          fontWeight: 400,
          lineHeight: 1.5,
          color: "#454545",
          overflow: "hidden",
          wordBreak: "break-word",
        },
        description,
      ),

      // サイト名：画像の下部に必ず固定する
      element(
        {
          position: "absolute",
          left: "78px",
          top: "530px",
          width: "1044px",
          height: "54px",
          boxSizing: "border-box",
          padding: "0 30px",
          alignItems: "center",
          justifyContent: "flex-start",
          borderRadius: "28px",
          backgroundColor: "#b39500",
          color: "#ffffff",
          fontSize: "23px",
          fontWeight: 700,
          lineHeight: 1.3,
          whiteSpace: "nowrap",
          overflow: "hidden",
        },
        SITE_NAME,
      ),
    ],
  );

  const svg = await satori(tree, {
    width: 1200,
    height: 630,
    fonts: [
      {
        name: "Noto Sans JP",
        data: font,
        weight: 400,
        style: "normal",
      },
      {
        name: "Noto Sans JP",
        data: font,
        weight: 700,
        style: "normal",
      },
    ],
  });

  return sharp(Buffer.from(svg))
    .png()
    .toBuffer();
}

createServer(async (req, res) => {
  try {
    const url = new URL(req.url || "/", "http://localhost");

    if (url.pathname !== "/") {
      res.writeHead(404).end();
      return;
    }

    const title = clean(
      url.searchParams.get("title"),
      "ページタイトル",
      100,
    );

    const description = clean(
      url.searchParams.get("description"),
      "ページの説明文",
      160,
    );

    const key = JSON.stringify([title, description]);

    const png = await getOrGenerate(key, title, description);

    res.writeHead(200, {
      "Content-Type": "image/png",
      "Content-Length": png.length,
      "Cache-Control": "public, max-age=3600, s-maxage=86400",
      "X-Content-Type-Options": "nosniff",
    });

    res.end(png);
  } catch (error) {
    console.error("OGP generation failed:", error);

    const statusCode = error?.statusCode === 503 ? 503 : 500;

    res.writeHead(statusCode, {
      "Content-Type": "text/plain; charset=utf-8",
      "Cache-Control": "no-store",
      ...(statusCode === 503 ? { "Retry-After": "1" } : {}),
    });

    res.end(
      statusCode === 503
        ? "OGP generation capacity exceeded"
        : "OGP generation failed",
    );
  }
}).listen(PORT, "127.0.0.1", () => {
  console.log(`OGP server listening on 127.0.0.1:${PORT}`);
});
