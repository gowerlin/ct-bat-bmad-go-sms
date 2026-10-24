# ct-skills-none

`compose.yaml` 的 `ct-skills` 建置 context 預設指到這裡：這個資料夾沒有任何 skill，所以映像不會內建 Control Tower skill。

要把 CT skill 放進映像，建置前設 `SMS_CT_SKILLS=<資料夾>`（shell 或 `.env`），資料夾裡要有 `control-tower/` 與 `ct-*/`（各含 `SKILL.md`），例如 CT 發行版的 `skills/`，或你自己的 `~/.claude/skills`（只會複製 `control-tower` 與 `ct-*`）。詳見 `README.md`〈dev container〉。
