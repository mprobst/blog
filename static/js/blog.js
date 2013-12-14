var _gaq = _gaq || [];
_gaq.push(['_setAccount', 'UA-21162656-1']);
_gaq.push(['_trackPageview']);

(function() {
  var ga = document.createElement('script'); ga.type = 'text/javascript'; ga.async = true;
  ga.src = ('https:' == document.location.protocol ? 'https://ssl' : 'http://www')
      + '.google-analytics.com/ga.js';
  var s = document.getElementsByTagName('script')[0]; s.parentNode.insertBefore(ga, s);
})();

$(document).ready(function() {
  $("pre").addClass("prettyprint")
  if (typeof(prettyPrint) !== 'undefined') prettyPrint();
})

$.ajax({
  url: "/blog/auth_check",
  success: function() {
    $(".admin_link").fadeIn(1000);
  },
  error: function() {
    // no-op, this user isn't authenticated.
  },
});

function toggle_comment_form() {
  var form = $("#add_comment");
  var link = $("#add_comment_link");
  if (form.height() != 0) {
    form.css("height", "0px");
    link.html(unescape("&#x25B8; Add a comment"));
  } else {
    var measure = $("#comment_form_measure");
    form.css("height", measure.height() + "px");
    link.html(unescape("&#x25BE; Add a comment"));
  }
}
