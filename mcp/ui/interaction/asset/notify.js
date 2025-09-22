(function(){
  function mcpNotifyAndClose(id, status){
    try{
      if (window.opener && !window.opener.closed) {
        window.opener.postMessage({type:'mcp:elicitation', elicitationId:id, status:status}, '*');
      }
    }catch(e){}
    setTimeout(function(){ try { window.close(); } catch(e){} }, 100);
  }
  window.mcpNotifyAndClose = mcpNotifyAndClose;

  // Auto-mode: if page URL has elicitationId and status, notify immediately.
  try{
    var params = new URLSearchParams(window.location.search || '');
    var id = params.get('elicitationId');
    var st = params.get('status');
    if (id && st) {
      mcpNotifyAndClose(id, st);
    }
  } catch(e){}
})();

