export function toCustomDate(time: any) {
  // 处理时间格式化-全部日期
  const date = new Date(time)
  const y = date.getFullYear()
  let m: any = date.getMonth() + 1
  m = m < 10 ? `0${m}` : m
  let d: any = date.getDate()
  d = d < 10 ? `0${d}` : d
  let h: any = date.getHours()
  h = h < 10 ? `0${h}` : h
  let minute: any = date.getMinutes()
  minute = minute < 10 ? `0${minute}` : minute
  let second: any = date.getSeconds()
  second = second < 10 ? `0${second}` : second
  return `${y}-${m}-${d} ${h}:${minute}:${second}`
}
