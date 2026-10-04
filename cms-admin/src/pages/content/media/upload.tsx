import { useRef, useState } from "react";
import { uploadMedia, getMedia, type MediaRecord } from "@/api/media";
import { Button } from "@/components/ui/button";
import { getErrorMessage } from "@/lib/api-error";
type Entry = { id:string; file:File; status:"waiting" | "uploading" | "succeeded" | "failed"; error:string };
export function MediaUpload({ disabled=false, onUploaded, onBusy }: { disabled?:boolean; onBusy?:(busy:boolean) => void; onUploaded:(file:MediaRecord) => void }) {
 const [entries,setEntries]=useState<Entry[]>([]);
 const busyCallback=useRef(onBusy); busyCallback.current=onBusy;
 const pending=useRef(0);
 async function upload(entry:Entry) {
 pending.current++; busyCallback.current?.(true);
 setEntries((items) => items.map((item) => item.id===entry.id ? {...item,status:"uploading",error:""} : item));
 try { const file=await uploadMedia(entry.file,{businessModule:"attachment"}); const media=await getMedia(file.id); setEntries((items) => items.map((item) => item.id===entry.id ? {...item,status:"succeeded",error:""} : item)); onUploaded(media); }
 catch(error) { setEntries((items) => items.map((item) => item.id===entry.id ? {...item,status:"failed",error:getErrorMessage(error,"上传失败；重试保持同一资源请求")} : item)); }
 finally { pending.current--; busyCallback.current?.(pending.current>0); }
 }
 return <div className="space-y-space-3"><label className="block font-medium">上传图片或附件<input aria-label="上传图片或附件" className="mt-space-2 block max-w-full" type="file" multiple disabled={disabled} onChange={(event) => { const added=Array.from(event.target.files ?? []).map((file):Entry => ({id:crypto.randomUUID(),file,status:"waiting",error:""})); setEntries((old) => [...old,...added]); added.forEach((entry) => void upload(entry)); event.target.value=""; }} /></label>
 <ul className="space-y-space-2">{entries.map((entry) => <li key={entry.id} className="flex flex-wrap gap-space-2 text-sm"><span className="break-all">{entry.file.name}</span><span role="status">{{ waiting:"等待上传",uploading:"上传中…",succeeded:"上传成功",failed:"上传失败" }[entry.status]}</span>{entry.error && <span role="alert" className="text-error">{entry.error}</span>}{entry.status==="failed" && <Button size="sm" disabled={disabled} onClick={() => void upload(entry)}>重试此文件</Button>}</li>)}</ul></div>;
}
