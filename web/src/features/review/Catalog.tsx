import type React from "react";
import { downloadExport } from "../../api/admin";
import { Heading } from "../../components/common/Heading";
import { Modal } from "../../components/common/Modal";
import { Table } from "../../components/common/Table";
import { FileRow } from "../../components/files/FileRow";
import { useData } from "../../hooks/useData";
import type { Obj } from "../../types/common";
import { CloudUpload, Download, LockKeyhole, Plus, Search } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";

export function Catalog({ api, refresh, notify, role, token }: any) {
  const [unused, setRefreshDummy] = useState(0);
  const loc = useLocation();
  const nav = useNavigate();
  const params = new URLSearchParams(loc.search);
  const linkedFileID = params.get("file_id") || "";
  const linkedArtifactID = params.get("artifact_id") || "";
  const [query, setQuery] = useState("");
  const [preservationFilter, setPreservationFilter] = useState("");
  const [policyFilter, setPolicyFilter] = useState("");
  const [publicationFilter, setPublicationFilter] = useState("");
  const [selectedIDs, setSelectedIDs] = useState<string[]>([]);
  const [bulkPolicy, setBulkPolicy] = useState("restricted");
  const [relatedRequestID, setRelatedRequestID] = useState("");
  const [modal, setModal] = useState(false);
  const [detail, setDetail] = useState<Obj | null>(null);
  const [metadata, setMetadata] = useState<Obj | null>(null);
  const [metaForm, setMetaForm] = useState({
    artifact_title: "",
    creator_name: "",
    artifact_source_url: "",
    artifact_description: "",
    original_filename: "",
    media_type: "",
    source_url: "",
  });
  const [artifact, setArtifact] = useState<Obj | null>(null);
  const [pendingFileId, setPendingFileId] = useState("");
  const [busy, setBusy] = useState(false);
  const [upload, setUpload] = useState<File | null>(null);
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const [sourceURL, setSourceURL] = useState("");
  const [acceptSourceTerms, setAcceptSourceTerms] = useState(false);
  const [listingURL, setListingURL] = useState("");
  const [acceptListingTerms, setAcceptListingTerms] = useState(false);
  const [discovering, setDiscovering] = useState(false);
  const [candidates, setCandidates] = useState<Obj[]>([]);
  const [selectedCandidates, setSelectedCandidates] = useState<string[]>([]);
  const [batchBusy, setBatchBusy] = useState(false);
  const [batchStatus, setBatchStatus] = useState("");
  const [priority, setPriority] = useState("3");
  const [policy, setPolicy] = useState("pending");
  const [pub, setPub] = useState(false);
  const [form, setForm] = useState({
    title: "",
    creator_name: "",
    source_url: "",
    description: "",
  });
  let filePath = "/files?limit=100";
  if (query.trim()) filePath += "&q=" + encodeURIComponent(query.trim());
  if (preservationFilter)
    filePath +=
      "&preservation_status=" + encodeURIComponent(preservationFilter);
  if (policyFilter)
    filePath += "&redistribution_policy=" + encodeURIComponent(policyFilter);
  if (publicationFilter)
    filePath += "&publication=" + encodeURIComponent(publicationFilter);
  if (linkedArtifactID)
    filePath += "&artifact_id=" + encodeURIComponent(linkedArtifactID);
  const f = useData(api, filePath, refresh + unused);
  const requestData = useData(
    api,
    "/public-requests?limit=100",
    refresh + unused,
  );
  const files: Obj[] = f.data.items || [];
  const requests: Obj[] = (requestData.data.items || []).filter(
    (x: Obj) =>
      x.status === "accepted" ||
      x.status === "in_review" ||
      x.status === "pending",
  );

  useEffect(() => {
    if (!linkedFileID || f.loading) return;
    let live = true;
    const local = (f.data.items || []).find((x: Obj) => x.id === linkedFileID);
    (local
      ? Promise.resolve(local)
      : api(`/files/${encodeURIComponent(linkedFileID)}`).then(
          (result: Obj) => result.file,
        ))
      .then((target: Obj) => {
        if (!live || !target) return;
        setDetail(target);
        setPolicy(target.redistribution_policy || "unknown");
        setPub(!!target.public_download_enabled);
      })
      .catch((e: Error) => live && notify(e.message))
      .finally(() => live && nav("/admin/review?tab=files", { replace: true }));
    return () => {
      live = false;
    };
  }, [linkedFileID, f.loading, f.data.items]);

  function openMetadata(x: Obj) {
    setMetadata(x);
    setMetaForm({
      artifact_title: x.artifact_title || "",
      creator_name: x.creator_name || "",
      artifact_source_url: x.artifact_source_url || "",
      artifact_description: x.artifact_description || "",
      original_filename: x.original_filename || "",
      media_type: x.media_type || "",
      source_url: x.source_url || "",
    });
  }
  async function saveMetadata(e: React.FormEvent) {
    e.preventDefault();
    if (!metadata) return;
    const reason = window.prompt("メタデータの変更理由を入力してください");
    if (!reason || reason.trim().length < 3) return;
    setBusy(true);
    try {
      await api(`/artifacts/${metadata.artifact_id}`, {
        method: "PATCH",
        body: JSON.stringify({
          title: metaForm.artifact_title,
          creator_name: metaForm.creator_name,
          source_url: metaForm.artifact_source_url,
          description: metaForm.artifact_description,
          reason: reason.trim(),
        }),
      });
      await api(`/files/${metadata.id}/metadata`, {
        method: "PATCH",
        body: JSON.stringify({
          original_filename: metaForm.original_filename,
          media_type: metaForm.media_type,
          source_url: metaForm.source_url,
          reason: reason.trim(),
        }),
      });
      notify("作品・ファイルのメタデータを更新しました");
      setMetadata(null);
      setRefreshDummy((x) => x + 1);
    } catch (e) {
      notify((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  async function runBulk(
    action: "policy" | "publication" | "delete",
    extra: Obj = {},
  ) {
    if (selectedIDs.length === 0) {
      notify("先にファイルを選択してください");
      return;
    }
    if (action === "delete" && !relatedRequestID) {
      notify("削除を承認した申請を選択してください");
      return;
    }
    if (action === "delete") {
      const linked = requests.find((r: Obj) => r.id === relatedRequestID);
      if (
        !linked ||
        linked.request_type !== "deletion" ||
        linked.status !== "accepted"
      ) {
        notify("承認済みの削除申請を選択してください");
        return;
      }
    }
    const reason = window.prompt(
      action === "delete"
        ? "削除処理の理由を入力してください"
        : "一括変更の理由を入力してください",
    );
    if (!reason || reason.trim().length < 3) return;
    try {
      const body: Obj = {
        file_ids: selectedIDs,
        action,
        reason: reason.trim(),
        ...extra,
      };
      if (relatedRequestID) body.request_id = relatedRequestID;
      const result = await api("/files/bulk-action", {
        method: "POST",
        body: JSON.stringify(body),
      });
      notify(`${result.count} 件を一括処理しました`);
      setSelectedIDs([]);
      setRefreshDummy((x) => x + 1);
    } catch (e) {
      notify((e as Error).message);
    }
  }
  async function createDeletionRequests() {
    if (selectedIDs.length === 0) return;
    const reason = window.prompt(
      "削除申請の理由を入力してください（5文字以上）",
    );
    if (!reason || reason.trim().length < 5) return;
    if (
      !window.confirm(
        `選択した ${selectedIDs.length} 件について削除申請を作成します。実際の削除は申請を承認した後に始まります。続行しますか？`,
      )
    )
      return;
    try {
      const result = await api("/admin/deletion-requests", {
        method: "POST",
        body: JSON.stringify({ file_ids: selectedIDs, reason: reason.trim() }),
      });
      notify(
        `${result.count} 件の削除申請を作成しました。申請・保全レビューで承認してください。`,
      );
      setSelectedIDs([]);
      nav("/admin/review?tab=requests");
    } catch (e) {
      notify((e as Error).message);
    }
  }
  function toggleSelected(id: string, checked: boolean) {
    setSelectedIDs((old) =>
      checked
        ? old.includes(id)
          ? old
          : [...old, id]
        : old.filter((x) => x !== id),
    );
  }
  function toggleAll(checked: boolean) {
    const selectable = files
      .filter((x: Obj) => !x.deletion_requested_at && !x.deleted_at)
      .map((x: Obj) => x.id);
    setSelectedIDs((old) =>
      checked
        ? Array.from(new Set([...old, ...selectable]))
        : old.filter((id) => !selectable.includes(id)),
    );
  }

  async function create(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    try {
      const a = await api("/artifacts", {
        method: "POST",
        body: JSON.stringify(form),
      });
      notify("作品を登録しました");
      setModal(false);
      setForm({ title: "", creator_name: "", source_url: "", description: "" });
      setArtifact(a);
      setPendingFileId("");
      setRefreshDummy((x) => x + 1);
    } catch (e) {
      notify((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  async function addFile(e: React.FormEvent) {
    e.preventDefault();
    const url = sourceURL.trim();
    if (
      !artifact ||
      (!upload && !url) ||
      (upload && url) ||
      (url && !acceptSourceTerms) ||
      policy === "pending"
    )
      return;
    setBusy(true);
    try {
      let fileId = pendingFileId;
      if (!fileId) {
        const file = await api(`/artifacts/${artifact.id}/files`, {
          method: "POST",
          body: JSON.stringify({
            original_filename: upload?.name || "取得待ち",
            size_bytes: upload?.size || 0,
            media_type: upload?.type || "application/octet-stream",
            source_url: url || undefined,
            redistribution_policy: policy,
            public_download_enabled: false,
          }),
        });
        fileId = file.id;
        setPendingFileId(fileId);
      }
      let ingestResult: any;
      if (upload) {
        const fd = new FormData();
        fd.append("file", upload);
        fd.append("priority", priority);
        ingestResult = await api(`/files/${fileId}/ingest`, {
          method: "POST",
          body: fd,
        });
      } else {
        ingestResult = await api(`/files/${fileId}/ingest-source`, {
          method: "POST",
          body: JSON.stringify({
            source_url: url,
            accept_terms: acceptSourceTerms,
            priority: Number(priority),
          }),
        });
      }
      const createdTasks = Array.isArray(ingestResult?.tasks)
        ? ingestResult.tasks.length
        : 0;
      notify(
        ingestResult?.source_fetch_task
          ? `取得タスクを ${ingestResult.source_fetch_task.node_name || "agent"} に登録しました。取得成功後に保存タスクが作成されます。`
          : createdTasks
            ? `登録しました。保存タスク ${createdTasks} 件を作成しました。`
            : "登録しました。保全処理を受け付けました。",
      );
      setArtifact(null);
      setPendingFileId("");
      setUpload(null);
      setSourceURL("");
      setAcceptSourceTerms(false);
      setListingURL("");
      setAcceptListingTerms(false);
      setCandidates([]);
      setSelectedCandidates([]);
      setBatchStatus("");
      setRefreshDummy((x) => x + 1);
    } catch (e) {
      notify((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  async function discoverListing() {
    if (!artifact || !listingURL.trim() || !acceptListingTerms) return;
    setDiscovering(true);
    setCandidates([]);
    setSelectedCandidates([]);
    setBatchStatus("一覧を確認しています…");
    try {
      const result = await api(
        `/artifacts/${artifact.id}/discover-source-files`,
        {
          method: "POST",
          body: JSON.stringify({
            listing_url: listingURL.trim(),
            accept_terms: acceptListingTerms,
          }),
        },
      );
      const found = (result.items || []) as Obj[];
      setCandidates(found);
      setSelectedCandidates(found.map((x: Obj) => x.url));
      setBatchStatus(
        `${result.pages_scanned} ページから ${found.length} 件を検出しました。` +
          (result.truncated
            ? "上限100件に達したため、残りは次回確認してください。"
            : ""),
      );
    } catch (e) {
      setBatchStatus((e as Error).message);
    } finally {
      setDiscovering(false);
    }
  }
  async function importSelected() {
    if (!artifact || selectedCandidates.length === 0 || policy === "pending")
      return;
    const chosen = candidates.filter((x: Obj) =>
      selectedCandidates.includes(x.url),
    );
    setBatchBusy(true);
    let ok = 0;
    let fetchTaskCount = 0;
    let taskCount = 0;
    const failures: string[] = [];
    for (let i = 0; i < chosen.length; i++) {
      const item = chosen[i];
      setBatchStatus(
        `取得タスク登録中 ${i + 1}/${chosen.length}: ${item.label || item.url}`,
      );
      try {
        const file = await api(`/artifacts/${artifact.id}/files`, {
          method: "POST",
          body: JSON.stringify({
            original_filename: (item.label || "取得待ち").slice(0, 500),
            size_bytes: 0,
            media_type: "application/octet-stream",
            source_url: item.url,
            redistribution_policy: policy,
          }),
        });
        const queued = await api(`/files/${file.id}/ingest-source`, {
          method: "POST",
          body: JSON.stringify({
            source_url: item.url,
            accept_terms: true,
            priority: Number(priority),
          }),
        });
        ok++;
        if (queued.source_fetch_task) fetchTaskCount++;
        taskCount += Array.isArray(queued?.tasks) ? queued.tasks.length : 0;
      } catch (e) {
        failures.push(`${item.label || item.url}: ${(e as Error).message}`);
      }
    }
    setBatchBusy(false);
    setSelectedCandidates([]);
    setCandidates([]);
    setBatchStatus(
      `完了: ${ok} 件受付、agent取得タスク ${fetchTaskCount} 件登録、保存タスク ${taskCount} 件作成、${failures.length} 件失敗。` +
        (fetchTaskCount
          ? "保存タスクは各 agent の取得成功後に作成されます。 "
          : "") +
        (failures.length ? `最初の失敗: ${failures[0]}` : ""),
    );
    notify(
      `一括保全: ${ok} 件受付、agent取得タスク ${fetchTaskCount} 件登録、失敗 ${failures.length} 件`,
    );
    setRefreshDummy((x) => x + 1);
  }
  async function savePolicy() {
    if (!detail) return;
    const reason = window.prompt("変更理由を入力してください");
    if (!reason || reason.trim().length < 3) return;
    try {
      await api(`/files/${detail.id}/policy`, {
        method: "PATCH",
        body: JSON.stringify({
          redistribution_policy: policy,
          reason: reason.trim(),
        }),
      });
      if (role === "admin" && pub !== detail.public_download_enabled) {
        await api(`/files/${detail.id}/publication`, {
          method: "PATCH",
          body: JSON.stringify({ enabled: pub, reason: reason.trim() }),
        });
      }
      notify("公開ポリシーを更新しました");
      setDetail(null);
      setRefreshDummy((x) => x + 1);
    } catch (e) {
      notify((e as Error).message);
    }
  }

  return (
    <>
      <Heading
        eyebrow=""
        title="作品・ファイル"
        desc="メタデータ、保全状態、公開条件を検索・一括管理"
        actions={
          <>
            <button
              className="secondary"
              onClick={() => downloadExport("csv", token, notify)}
            >
              <Download size={16} /> CSV出力
            </button>
            {role === "admin" && (
              <button className="primary" onClick={() => setModal(true)}>
                <Plus size={16} /> 作品を登録
              </button>
            )}
          </>
        }
      />
      <div className="filter-bar file-filter-bar">
        <div className="search-box">
          <Search size={17} />
          <input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="作品名・ファイル名・作者・IDで検索"
          />
        </div>
        <select
          aria-label="保全状態で絞り込み"
          value={preservationFilter}
          onChange={(e) => setPreservationFilter(e.target.value)}
        >
          <option value="">すべての保全状態</option>
          <option value="uncollected">未取得</option>
          <option value="queued">待機中</option>
          <option value="stored">保存済み</option>
          <option value="verified">検証済み</option>
          <option value="degraded">劣化</option>
          <option value="missing">実体不明</option>
          <option value="error">エラー</option>
        </select>
        <select
          aria-label="公開条件で絞り込み"
          value={policyFilter}
          onChange={(e) => setPolicyFilter(e.target.value)}
        >
          <option value="">すべての公開条件</option>
          <option value="allowed">再配布可</option>
          <option value="restricted">制限付き</option>
          <option value="in_progress">確認中</option>
          <option value="pending">未確認</option>
          <option value="unknown">不明</option>
        </select>
        <select
          aria-label="公開状態で絞り込み"
          value={publicationFilter}
          onChange={(e) => setPublicationFilter(e.target.value)}
        >
          <option value="">公開状態：すべて</option>
          <option value="public">公開中</option>
          <option value="private">非公開</option>
          <option value="restricted">制限付き</option>
          <option value="pending">未確認</option>
          <option value="unknown">条件不明</option>
        </select>
        <span className="result-count">{f.data.total ?? files.length} 件</span>
      </div>
      {selectedIDs.length > 0 && role !== "viewer" && (
        <section className="bulk-toolbar panel">
          <div className="bulk-toolbar-title">
            <b>{selectedIDs.length} 件を選択中</b>
            <button className="small-btn" onClick={() => setSelectedIDs([])}>
              選択解除
            </button>
          </div>
          <div className="bulk-toolbar-controls">
            <label>
              関連申請
              <select
                value={relatedRequestID}
                onChange={(e) => setRelatedRequestID(e.target.value)}
              >
                <option value="">申請に紐づけない</option>
                {requests.map((r: Obj) => (
                  <option key={r.id} value={r.id}>
                    {r.request_type === "deletion"
                      ? "削除"
                      : r.request_type === "takedown"
                        ? "公開停止"
                        : r.request_type === "preservation"
                          ? "保全"
                          : r.request_type === "redistribution_assistance"
                            ? "再配布相談"
                            : r.request_type === "publisher_assistance"
                              ? "配布代行"
                              : "移設先"}{" "}
                    /{" "}
                    {r.linked_file_name ||
                      r.linked_artifact_title ||
                      r.title ||
                      String(r.id).slice(0, 8)}{" "}
                    /{" "}
                    {r.status === "accepted"
                      ? "承認"
                      : r.status === "in_review"
                        ? "確認中"
                        : "未処理"}
                  </option>
                ))}
              </select>
            </label>
            <label>
              公開条件
              <select
                value={bulkPolicy}
                onChange={(e) => setBulkPolicy(e.target.value)}
              >
                <option value="allowed">再配布可</option>
                <option value="restricted">制限付き</option>
                <option value="in_progress">確認中</option>
                <option value="pending">未確認</option>
                <option value="unknown">不明</option>
              </select>
            </label>
            <button
              className="secondary"
              onClick={() =>
                runBulk("policy", { redistribution_policy: bulkPolicy })
              }
            >
              公開条件を一括変更
            </button>
            {role === "admin" && (
              <>
                <button
                  className="primary"
                  onClick={() => runBulk("publication", { enabled: true })}
                >
                  一括公開
                </button>
                <button
                  className="secondary"
                  onClick={() => runBulk("publication", { enabled: false })}
                >
                  一括公開停止
                </button>
                <button className="danger-btn" onClick={createDeletionRequests}>
                  削除申請を作成
                </button>
                <button
                  className="danger-btn"
                  onClick={() => runBulk("delete")}
                >
                  承認済み申請で削除
                </button>
              </>
            )}
          </div>
          <p className="muted">
            「削除申請を作成」は申請を未処理で登録します。申請・保全レビューで管理者が承認するとノード削除タスクが開始します。承認済みの申請を使った削除も可能です。
          </p>
        </section>
      )}
      <section className="panel table-panel">
        <Table
          headers={[
            "",
            "作品 / ファイル",
            "保全状態",
            "公開条件 / 状態",
            "サイズ",
            "登録日時",
            "操作",
          ]}
        >
          {f.loading ? (
            <tr>
              <td colSpan={7} className="empty">
                読み込み中…
              </td>
            </tr>
          ) : f.error ? (
            <tr>
              <td colSpan={7} className="empty">
                {f.error}
              </td>
            </tr>
          ) : files.length ? (
            <>
              <tr className="select-all-row">
                <td>
                  <input
                    type="checkbox"
                    aria-label="表示中のファイルをすべて選択"
                    checked={
                      files.filter(
                        (x: Obj) => !x.deletion_requested_at && !x.deleted_at,
                      ).length > 0 &&
                      files
                        .filter(
                          (x: Obj) => !x.deletion_requested_at && !x.deleted_at,
                        )
                        .every((x: Obj) => selectedIDs.includes(x.id))
                    }
                    disabled={role === "viewer"}
                    onChange={(e) => toggleAll(e.target.checked)}
                  />
                </td>
                <td colSpan={6} className="muted">
                  表示中の {files.length} 件を選択
                </td>
              </tr>
              {files.map((x: Obj) => (
                <FileRow
                  key={x.id}
                  x={x}
                  role={role}
                  isSelected={selectedIDs.includes(x.id)}
                  toggleSelected={toggleSelected}
                  setDetail={setDetail}
                  setPolicy={setPolicy}
                  setPub={setPub}
                  openMetadata={openMetadata}
                  token={token}
                />
              ))}
            </>
          ) : (
            <tr>
              <td colSpan={7} className="empty">
                ファイルがありません
              </td>
            </tr>
          )}
        </Table>
      </section>
      {modal && (
        <Modal title="作品を登録" close={() => setModal(false)}>
          <form onSubmit={create} className="form-stack">
            <label>
              作品名 *
              <input
                required
                value={form.title}
                onChange={(e) => setForm({ ...form, title: e.target.value })}
              />
            </label>
            <label>
              作者・配布者
              <input
                value={form.creator_name}
                onChange={(e) =>
                  setForm({ ...form, creator_name: e.target.value })
                }
              />
            </label>
            <label>
              元ページURL *
              <input
                required
                type="url"
                placeholder="https://www.uploader.jp/..."
                value={form.source_url}
                onChange={(e) =>
                  setForm({ ...form, source_url: e.target.value })
                }
              />
            </label>
            <label>
              説明
              <textarea
                rows={3}
                value={form.description}
                onChange={(e) =>
                  setForm({ ...form, description: e.target.value })
                }
              />
            </label>
            <div className="modal-actions">
              <button
                type="button"
                className="secondary"
                onClick={() => setModal(false)}
              >
                キャンセル
              </button>
              <button className="primary" disabled={busy}>
                {busy ? "登録中…" : "次へ"}
              </button>
            </div>
          </form>
        </Modal>
      )}
      {artifact && (
        <Modal
          title="ファイルを追加・保全"
          close={() => {
            setArtifact(null);
            setPendingFileId("");
            setListingURL("");
            setAcceptListingTerms(false);
            setCandidates([]);
            setSelectedCandidates([]);
            setBatchStatus("");
          }}
        >
          <form className="form-stack" onSubmit={addFile}>
            <p className="muted">
              作品: <b>{artifact.title}</b>
            </p>
            <label>
              手元のファイル
              <input
                ref={fileInputRef}
                type="file"
                onChange={(e) => {
                  setUpload(e.target.files?.[0] || null);
                  if (e.target.files?.[0]) {
                    setSourceURL("");
                    setAcceptSourceTerms(false);
                    setListingURL("");
                    setAcceptListingTerms(false);
                    setCandidates([]);
                    setSelectedCandidates([]);
                    setPendingFileId("");
                  }
                }}
              />
              <small>最大5 GiB。保存先は2ノード必要です。</small>
            </label>
            <label>
              または uploader.jp のダウンロードURL
              <input
                type="url"
                value={sourceURL}
                onChange={(e) => {
                  setSourceURL(e.target.value);
                  if (e.target.value.trim()) {
                    setUpload(null);
                    setListingURL("");
                    setAcceptListingTerms(false);
                    setCandidates([]);
                    setSelectedCandidates([]);
                    if (fileInputRef.current) fileInputRef.current.value = "";
                  } else setPendingFileId("");
                }}
                placeholder="https://uu.getuploader.com/名前/download/123"
              />
              <small>
                個別ファイル用。パスワードや画像認証の回避は行いません。
              </small>
            </label>
            {sourceURL.trim() && (
              <label className="check-label">
                <input
                  type="checkbox"
                  checked={acceptSourceTerms}
                  onChange={(e) => setAcceptSourceTerms(e.target.checked)}
                />{" "}
                uploader.jp の利用規約を確認し、同意しました
              </label>
            )}
            <div className="form-divider" />
            <label>
              またはアップローダーの一覧URL
              <input
                type="url"
                value={listingURL}
                onChange={(e) => {
                  setListingURL(e.target.value);
                  if (e.target.value.trim()) {
                    setSourceURL("");
                    setAcceptSourceTerms(false);
                    setUpload(null);
                    setPendingFileId("");
                    if (fileInputRef.current) fileInputRef.current.value = "";
                  }
                  setCandidates([]);
                  setSelectedCandidates([]);
                  setBatchStatus("");
                }}
                placeholder="https://uu.getuploader.com/名前/"
              />
              <small>
                一覧ページを調べ、掲載ファイルを選んで順番に取得します（最大100件・最大10ページ）。
              </small>
            </label>
            {listingURL.trim() && (
              <label className="check-label">
                <input
                  type="checkbox"
                  checked={acceptListingTerms}
                  onChange={(e) => setAcceptListingTerms(e.target.checked)}
                />{" "}
                uploader.jp の利用規約を確認しました
              </label>
            )}
            <div className="modal-actions">
              <button
                type="button"
                className="secondary"
                disabled={
                  !listingURL.trim() ||
                  !acceptListingTerms ||
                  discovering ||
                  batchBusy
                }
                onClick={discoverListing}
              >
                {discovering ? "確認中…" : "一覧を調べる"}
              </button>
            </div>
            {batchStatus && <p className="muted">{batchStatus}</p>}
            {candidates.length > 0 && (
              <div className="candidate-list">
                <div className="candidate-list-head">
                  <b>検出したファイル ({candidates.length})</b>
                  <button
                    type="button"
                    className="small-btn"
                    onClick={() =>
                      setSelectedCandidates(
                        selectedCandidates.length === candidates.length
                          ? []
                          : candidates.map((x: Obj) => x.url),
                      )
                    }
                  >
                    {selectedCandidates.length === candidates.length
                      ? "選択解除"
                      : "全件選択"}
                  </button>
                </div>
                {candidates.map((x: Obj) => (
                  <label className="candidate-row" key={x.url}>
                    <input
                      type="checkbox"
                      checked={selectedCandidates.includes(x.url)}
                      onChange={(e) =>
                        setSelectedCandidates(
                          e.target.checked
                            ? [...selectedCandidates, x.url]
                            : selectedCandidates.filter((u) => u !== x.url),
                        )
                      }
                    />
                    <span>
                      {x.label || x.url}
                      <small>{x.url}</small>
                    </span>
                  </label>
                ))}
                <button
                  type="button"
                  className="primary wide"
                  disabled={
                    batchBusy ||
                    selectedCandidates.length === 0 ||
                    policy === "pending"
                  }
                  onClick={importSelected}
                >
                  {batchBusy
                    ? "取得タスク登録中…"
                    : `選択した ${selectedCandidates.length} 件の取得タスクを登録`}
                </button>
              </div>
            )}
            <label>
              公開条件
              <select
                value={policy}
                disabled={busy || batchBusy}
                onChange={(e) => setPolicy(e.target.value)}
              >
                <option value="pending">未確認（保全不可）</option>
                <option value="unknown">不明</option>
                <option value="in_progress">確認中</option>
                <option value="allowed">許可済み</option>
                <option value="restricted">制限あり</option>
              </select>
            </label>
            <p className="muted">
              保全と一般公開は別の判定です。公開条件が未確認のファイルは保全を開始できません。
            </p>
            <label>
              保存優先度（0〜5）
              <select
                value={priority}
                onChange={(e) => setPriority(e.target.value)}
              >
                {[5, 4, 3, 2, 1, 0].map((x) => (
                  <option key={x} value={x}>
                    {x}
                  </option>
                ))}
              </select>
            </label>
            <div className="modal-actions">
              <button
                type="button"
                className="secondary"
                onClick={() => {
                  setArtifact(null);
                  setPendingFileId("");
                  setListingURL("");
                  setAcceptListingTerms(false);
                  setCandidates([]);
                  setSelectedCandidates([]);
                  setBatchStatus("");
                }}
              >
                キャンセル
              </button>
              <button
                className="primary"
                disabled={
                  busy ||
                  (!upload && !sourceURL.trim()) ||
                  (!!upload && !!sourceURL.trim()) ||
                  (!!sourceURL.trim() && !acceptSourceTerms) ||
                  policy === "pending"
                }
              >
                <CloudUpload size={16} />
                {busy
                  ? "処理中…"
                  : sourceURL.trim()
                    ? "取得して保全開始"
                    : "登録して保全開始"}
              </button>
            </div>
          </form>
        </Modal>
      )}
      {detail && (
        <Modal title="公開設定" close={() => setDetail(null)}>
          <div className="detail-summary">
            <b>{detail.artifact_title || detail.original_filename}</b>
            <small>{detail.original_filename}</small>
            <small>{detail.id}</small>
            {detail.source_url && (
              <small>
                <a href={detail.source_url} target="_blank" rel="noreferrer">
                  取得元URLを開く
                </a>
              </small>
            )}
            {detail.deleted_at && (
              <div className="policy-warning">
                <LockKeyhole size={16} /> 削除済みのファイルです。
              </div>
            )}
            {detail.deletion_requested_at && !detail.deleted_at && (
              <div className="policy-warning">
                <LockKeyhole size={16} /> ノードへの削除タスクを処理中です。
              </div>
            )}
            <div className="policy-warning">
              <LockKeyhole size={16} /> 公開は権利確認後に有効にしてください。
            </div>
          </div>
          <div className="form-stack">
            <label>
              公開条件
              <select
                value={policy}
                onChange={(e) => setPolicy(e.target.value)}
              >
                <option value="unknown">不明</option>
                <option value="pending">未確認（保全不可）</option>
                <option value="in_progress">確認中</option>
                <option value="allowed">許可済み</option>
                <option value="restricted">制限あり</option>
              </select>
            </label>
            <label className="check-label">
              <input
                type="checkbox"
                checked={pub}
                disabled={role !== "admin"}
                onChange={(e) => setPub(e.target.checked)}
              />{" "}
              一般公開する
            </label>
            <p className="muted">
              公開には再配布可の確認、検証済み状態、オンライン保存先が必要です。
            </p>
            <div className="modal-actions">
              <button className="secondary" onClick={() => setDetail(null)}>
                閉じる
              </button>
              <button
                className="primary"
                onClick={savePolicy}
                disabled={!!detail.deleted_at || !!detail.deletion_requested_at}
              >
                設定を保存
              </button>
            </div>
          </div>
        </Modal>
      )}
      {metadata && (
        <Modal
          title="作品・ファイルのメタデータ"
          close={() => setMetadata(null)}
        >
          <form className="form-stack" onSubmit={saveMetadata}>
            <label>
              作品名 *
              <input
                required
                value={metaForm.artifact_title}
                onChange={(e) =>
                  setMetaForm({ ...metaForm, artifact_title: e.target.value })
                }
              />
            </label>
            <label>
              作者・配布者
              <input
                value={metaForm.creator_name}
                onChange={(e) =>
                  setMetaForm({ ...metaForm, creator_name: e.target.value })
                }
              />
            </label>
            <label>
              作品の元ページURL *
              <input
                required
                type="url"
                value={metaForm.artifact_source_url}
                onChange={(e) =>
                  setMetaForm({
                    ...metaForm,
                    artifact_source_url: e.target.value,
                  })
                }
              />
            </label>
            <label>
              作品の説明
              <textarea
                rows={3}
                value={metaForm.artifact_description}
                onChange={(e) =>
                  setMetaForm({
                    ...metaForm,
                    artifact_description: e.target.value,
                  })
                }
              />
            </label>
            <div className="form-divider" />
            <label>
              ファイル名 *
              <input
                required
                value={metaForm.original_filename}
                onChange={(e) =>
                  setMetaForm({
                    ...metaForm,
                    original_filename: e.target.value,
                  })
                }
              />
            </label>
            <label>
              MIME タイプ
              <input
                value={metaForm.media_type}
                onChange={(e) =>
                  setMetaForm({ ...metaForm, media_type: e.target.value })
                }
                placeholder="application/zip"
              />
            </label>
            <label>
              ファイル取得元URL
              <input
                type="url"
                value={metaForm.source_url}
                onChange={(e) =>
                  setMetaForm({ ...metaForm, source_url: e.target.value })
                }
              />
            </label>
            <p className="muted">
              作者・作品名は作品レコードを更新するため、同じ作品に属するファイルの表示にも反映されます。
            </p>
            <div className="modal-actions">
              <button
                type="button"
                className="secondary"
                onClick={() => setMetadata(null)}
              >
                キャンセル
              </button>
              <button className="primary" disabled={busy}>
                {busy ? "保存中…" : "メタデータを保存"}
              </button>
            </div>
          </form>
        </Modal>
      )}
    </>
  );
}
