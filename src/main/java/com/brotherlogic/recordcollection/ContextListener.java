package com.brotherlogic.recordcollection;

import java.util.TreeMap;

import javax.servlet.ServletContextEvent;
import javax.servlet.ServletContextListener;

import org.apache.log4j.ConsoleAppender;
import org.apache.log4j.Level;
import org.apache.log4j.Logger;
import org.apache.log4j.PatternLayout;

import org.scribe.builder.ServiceBuilder;
import org.scribe.oauth.OAuthService;
import org.scribe.model.Token;

public class ContextListener implements ServletContextListener {

    private final String CALLBACK_URL = "http://localhost";
    
    @Override
    public void contextDestroyed(ServletContextEvent arg0) {
    }
    
    @Override
    public void contextInitialized(ServletContextEvent arg0) {        
        ConsoleAppender console = new ConsoleAppender(); // create appender
        String pattern = "[%c] [%d{dd MMM yyyy - hh:mm:ss}] %5p - %m %n";
        console.setLayout(new PatternLayout(pattern));
        console.setThreshold(Level.INFO);
        console.activateOptions();
        Logger.getRootLogger().removeAllAppenders();
        Logger.getRootLogger().addAppender(console);

        OAuthService service = new ServiceBuilder()
	    .provider(DiscogsApi.class)
	    .apiKey(System.getenv("discogskey"))
	    .apiSecret(System.getenv("discogssecret"))
	    .callback(CALLBACK_URL)
	    .build();

        arg0.getServletContext().setAttribute("config",new Config(System.getenv("discogskey"),System.getenv("discogssecret"),service));
        arg0.getServletContext().setAttribute("token_map",new TreeMap<String,Token>());
        arg0.getServletContext().setAttribute("auth_tokens",new TreeMap<String,Token>());
    }
}
