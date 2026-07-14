# SquirtleChat API smoke test
# Requires: .\scripts\start-backend.ps1

param(
    [string]$Base = "http://localhost:8080/api/v1"
)

$ErrorActionPreference = "Stop"

function Invoke-Api {
    param([string]$Method, [string]$Path, [hashtable]$Body, [string]$Token)
    $headers = @{ "Content-Type" = "application/json" }
    if ($Token) { $headers["Authorization"] = "Bearer $Token" }
    $uri = "$Base$Path"
    if ($Body) {
        return Invoke-RestMethod -Method $Method -Uri $uri -Headers $headers -Body ($Body | ConvertTo-Json)
    }
    return Invoke-RestMethod -Method $Method -Uri $uri -Headers $headers
}

Write-Host "== health =="
Invoke-RestMethod "http://localhost:8080/health" | Out-Null
Write-Host "health OK"

$suffix = [guid]::NewGuid().ToString("N").Substring(0, 8)
$userA = "smoke_a_$suffix"
$userB = "smoke_b_$suffix"
$pass = "test1234"

Write-Host "== register =="
$regA = Invoke-Api -Method POST -Path "/auth/register" -Body @{ username = $userA; password = $pass; nickname = "A" }
$regB = Invoke-Api -Method POST -Path "/auth/register" -Body @{ username = $userB; password = $pass; nickname = "B" }
$tokenA = $regA.data.tokens.access_token
$tokenB = $regB.data.tokens.access_token
$idB = $regB.data.user.id
Write-Host "register OK"

Write-Host "== search =="
$search = Invoke-Api -Method GET -Path "/users/search?q=$userB" -Token $tokenA
if (-not $search.data.users) { throw "search failed" }
Write-Host "search OK"

Write-Host "== friend =="
Invoke-Api -Method POST -Path "/friends/request" -Body @{ to_user_id = $idB } -Token $tokenA | Out-Null
$pending = Invoke-Api -Method GET -Path "/friends/requests" -Token $tokenB
$reqId = $pending.data.requests[0].id
Invoke-Api -Method POST -Path "/friends/request/$reqId/accept" -Token $tokenB | Out-Null
Write-Host "friend OK"

Write-Host "== profile =="
Invoke-Api -Method PUT -Path "/users/me" -Body @{ nickname = "A2"; gender = 1 } -Token $tokenA | Out-Null
Invoke-Api -Method PUT -Path "/users/me/privacy" -Body @{
    show_nickname = $false; show_gender = $false; show_birthday = $false; show_avatar = $false
} -Token $tokenB | Out-Null
$pub = Invoke-Api -Method GET -Path "/users/$idB" -Token $tokenA
if ($pub.data.user.nickname) { throw "privacy leak: nickname visible" }
Write-Host "profile + privacy OK"

Write-Host "== friends list =="
$friends = Invoke-Api -Method GET -Path "/friends" -Token $tokenA
if (-not $friends.data.friends) { throw "friends list failed" }
Write-Host "friends OK"

Write-Host "== friend remark =="
$idA = $regA.data.user.id
$remarkText = "buddy_$suffix"
Invoke-Api -Method PUT -Path "/friends/$idB/remark" -Body @{ remark = $remarkText } -Token $tokenA | Out-Null
$friends2 = Invoke-Api -Method GET -Path "/friends" -Token $tokenA
$remarked = $friends2.data.friends | Where-Object { "$($_.id)" -eq "$idB" }
if (-not $remarked -or $remarked.remark -ne $remarkText) { throw "friend remark failed" }
Write-Host "friend remark OK"

Write-Host "== group =="
$grp = Invoke-Api -Method POST -Path "/groups" -Body @{ name = "smoke_group"; invite_friend_ids = @($idB) } -Token $tokenA
$convID = $grp.data.conversation_id
$groupNo = $grp.data.group_no
if (-not $convID) { throw "group conversation_id missing" }
if (-not $groupNo -or $groupNo.Length -lt 8) { throw "group_no missing or invalid (expected ~10 digits)" }
Write-Host "group create OK (group_no=$groupNo)"

Write-Host "== group discover =="
$searchNo = Invoke-Api -Method GET -Path "/groups/discover?q=$groupNo" -Token $tokenB
if (-not $searchNo.data.groups) { throw "group discover by no failed" }
$searchName = Invoke-Api -Method GET -Path "/groups/discover?q=smoke_group" -Token $tokenB
if (-not $searchName.data.groups) { throw "group discover by name failed" }
Write-Host "group discover OK"

Write-Host "== group invite accept =="
$invitesB = Invoke-Api -Method GET -Path "/groups/invitations" -Token $tokenB
if (-not $invitesB.data.invitations -or $invitesB.data.invitations.Count -lt 1) { throw "group invitation missing for B" }
$inviteId = $invitesB.data.invitations[0].id
Invoke-Api -Method POST -Path "/groups/invitations/$inviteId/accept" -Token $tokenB | Out-Null
Write-Host "group invite accept OK"

Write-Host "== face-to-face =="
$faceCode = "{0:D4}" -f (Get-Random -Minimum 1000 -Maximum 9999)
$face = Invoke-Api -Method POST -Path "/groups/face-to-face/start" -Body @{ code = $faceCode } -Token $tokenA
if (-not $face.data.face_code -or $face.data.face_code -ne $faceCode) { throw "face code missing" }
$regC = Invoke-Api -Method POST -Path "/auth/register" -Body @{ username = "smoke_c_$suffix"; password = $pass; nickname = "C" }
$tokenC = $regC.data.tokens.access_token
$faceJoin = Invoke-Api -Method POST -Path "/groups/face-to-face/join" -Body @{ code = $faceCode } -Token $tokenC
if (-not $faceJoin.data.conversation_id) { throw "face join failed" }
Write-Host "face-to-face OK"

Write-Host "== conversations =="
$convs = Invoke-Api -Method GET -Path "/conversations" -Token $tokenA
Write-Host "conversations OK"

Write-Host "== group detail =="
$groupId = ($convID -replace '^g_', '')
$detail = Invoke-Api -Method GET -Path "/groups/$groupId" -Token $tokenA
if (-not $detail.data.members -or $detail.data.members.Count -lt 2) { throw "group members missing" }
Write-Host "group detail OK"

Write-Host "== group pending invites =="
$pendingInv = Invoke-Api -Method GET -Path "/groups/$groupId/invitations" -Token $tokenA
if ($null -eq $pendingInv.data.invitations) { throw "group pending invites response failed" }
Write-Host "group pending invites OK"

Write-Host "== group kick =="
$regD = Invoke-Api -Method POST -Path "/auth/register" -Body @{ username = "smoke_d_$suffix"; password = $pass; nickname = "D" }
$idD = $regD.data.user.id
$tokenD = $regD.data.tokens.access_token
Invoke-Api -Method POST -Path "/friends/request" -Body @{ to_user_id = $idD } -Token $tokenA | Out-Null
$pendingD = Invoke-Api -Method GET -Path "/friends/requests" -Token $tokenD
$reqD = $pendingD.data.requests[0].id
Invoke-Api -Method POST -Path "/friends/request/$reqD/accept" -Token $tokenD | Out-Null
Invoke-Api -Method POST -Path "/groups/$groupId/invites" -Body @{ user_ids = @([int64]$idD) } -Token $tokenA | Out-Null
$invD = Invoke-Api -Method GET -Path "/groups/invitations" -Token $tokenD
$invDId = $invD.data.invitations[0].id
Invoke-Api -Method POST -Path "/groups/invitations/$invDId/accept" -Token $tokenD | Out-Null
$beforeKick = Invoke-Api -Method GET -Path "/groups/$groupId" -Token $tokenA
Invoke-Api -Method DELETE -Path "/groups/$groupId/members/$idD" -Token $tokenA | Out-Null
$detailKick = Invoke-Api -Method GET -Path "/groups/$groupId" -Token $tokenA
if ($detailKick.data.members.Count -ge $beforeKick.data.members.Count) { throw "kick member failed" }
$stillIn = $detailKick.data.members | Where-Object { "$($_.id)" -eq "$idD" }
if ($stillIn) { throw "kicked member still in group" }
Write-Host "group kick OK"

Write-Host "== group notice =="
$noticeText = "notice_$suffix"
Invoke-Api -Method PUT -Path "/groups/$groupId/notice" -Body @{ notice = $noticeText } -Token $tokenA | Out-Null
$detail2 = Invoke-Api -Method GET -Path "/groups/$groupId" -Token $tokenA
if ($detail2.data.notice -ne $noticeText) { throw "group notice not saved" }
$denied = Invoke-Api -Method PUT -Path "/groups/$groupId/notice" -Body @{ notice = "hack" } -Token $tokenB
if ($denied.code -eq 0) { throw "non-owner should not set notice" }
Write-Host "group notice OK"

Write-Host "== message search =="
$emptySearch = Invoke-Api -Method GET -Path "/conversations/$convID/messages/search?q=hello&limit=10" -Token $tokenA
if ($null -eq $emptySearch.data.messages) { throw "message search empty response failed" }
$keyword = "smoke_kw_$suffix"
$msgId = [int64]([DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()) * 1000 + (Get-Random -Maximum 999)
$clientMsgId = [guid]::NewGuid().ToString()
$sql = "INSERT INTO messages (id, conversation_id, from_user_id, seq, msg_type, content, client_msg_id) VALUES ($msgId, '$convID', $idA, 1, 1, 'hello $keyword world', '$clientMsgId');"
$prevEap = $ErrorActionPreference
$ErrorActionPreference = "Continue"
$sql | & mysql -u squirtle "-psquirtle123" squirtlechat 2>&1 | Out-Null
$mysqlExit = $LASTEXITCODE
$ErrorActionPreference = $prevEap
if ($mysqlExit -ne 0) { throw "seed message insert failed" }
$hit = Invoke-Api -Method GET -Path "/conversations/$convID/messages/search?q=$keyword&limit=10" -Token $tokenA
if (-not $hit.data.messages -or $hit.data.messages.Count -lt 1) { throw "message search hit failed" }
$found = $hit.data.messages | Where-Object { $_.content -like "*$keyword*" }
if (-not $found) { throw "message search content mismatch" }
$around = Invoke-Api -Method GET -Path "/conversations/$convID/messages?around_seq=1&limit=20" -Token $tokenA
if (-not $around.data.messages -or $around.data.messages.Count -lt 1) { throw "around_seq list failed" }
Write-Host "message search OK"

Write-Host "== file upload =="
$boundary = [guid]::NewGuid().ToString()
$lf = "`r`n"
$uploadBody = [Text.Encoding]::UTF8.GetBytes(
  ("--$boundary", "Content-Disposition: form-data; name=`"file`"; filename=`"smoke_$suffix.txt`"", "Content-Type: text/plain", "", "smoke upload", "--$boundary--") -join $lf
)
$uploadHeaders = @{ Authorization = "Bearer $tokenA" }
$uploadResp = Invoke-RestMethod -Method POST -Uri "$Base/files/upload" -Headers $uploadHeaders -ContentType "multipart/form-data; boundary=$boundary" -Body $uploadBody
if ($uploadResp.code -ne 0 -or -not $uploadResp.data.url) { throw "file upload failed" }
if ($uploadResp.data.url -notlike "/uploads/*") { throw "file upload url should be /uploads/*" }
$dlUrl = "http://localhost:8080$($uploadResp.data.url)"
$dl = Invoke-WebRequest -Uri $dlUrl -UseBasicParsing
if ($dl.StatusCode -ne 200 -or $dl.Content -notlike "*smoke upload*") { throw "file download via /uploads failed" }
Write-Host "file upload OK"

Write-Host "== agent =="
$agentInfo = Invoke-Api -Method GET -Path "/agent/info" -Token $tokenA
if (-not $agentInfo.data.user -or $agentInfo.data.user.username -ne "squirtle_ai") { throw "agent info failed" }
Write-Host "agent OK"

Write-Host ""
Write-Host "ALL SMOKE TESTS PASSED"
