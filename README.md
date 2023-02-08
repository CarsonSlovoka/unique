# Unique

移除重複的檔案

以檔案md5為主，若有重複就只留下一個。

其中保留的那一個可以依照

- cTime: 保留建立日期最早的檔案
- len: 保留檔案路徑最短者

## [設定檔](v1/.unique.json)

```json5
{
  "wkDir": "./testDir", // C:\\...\\images // 絕對路徑或者相對路徑都可以
  // "suffixes": ["*"], // 代表不做判斷，所有副檔名都會列入判斷
  "suffixes": [
    ".png",
    ".jpg"
  ], // 只對png與jpg做判斷
  "condition": "cTime", // len, cTime
}
```

注意比較條件是以md5為主，即便兩個不同副檔名的檔案，只要他們的md5數值為準，就會列入考量，例如
```
aa.txt
bbb.png
若兩者相同md5數值都相同
在len的模式下只會保留aa.txt
```

